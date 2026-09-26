// Package image pulls a remote disk image and streams it onto a block device.
package image

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"time"

	"github.com/dustin/go-humanize"
	"golang.org/x/sync/errgroup"
)

// Write streams sourceImage onto destinationDevice and reports what it did.
//
// The transfer runs as a pipeline: an HTTP read-ahead ring, a decompressor, a
// chunker emitting block-aligned chunks, and a pool of workers issuing pwrite.
// The stages overlap, so network, decompression and disk IO proceed at once
// instead of taking turns.
func Write(ctx context.Context, sourceImage, destinationDevice string, opts Options) (Result, error) {
	log := opts.Logger
	if log == nil {
		log = slog.New(slog.NewTextHandler(io.Discard, nil))
	}

	// ModeDiff reads the destination back, so it cannot use O_WRONLY.
	flags := os.O_WRONLY
	if opts.Mode == ModeDiff {
		flags = os.O_RDWR
	}
	fileOut, err := os.OpenFile(destinationDevice, flags, 0o644)
	if err != nil {
		return Result{}, fmt.Errorf("opening %s: %w", destinationDevice, err)
	}
	// Not deferred: Close reports errors that matter, and the shutdown sequence
	// below has to order sync, partition reread and close explicitly.
	closed := false
	closeOut := func() error {
		if closed {
			return nil
		}
		closed = true
		if cerr := fileOut.Close(); cerr != nil {
			return fmt.Errorf("closing %s: %w", destinationDevice, cerr)
		}
		return nil
	}

	geom, err := ProbeGeometry(fileOut)
	if err != nil {
		_ = closeOut()
		return Result{}, err
	}
	opts.applyDefaults(geom)

	res, err := writeTo(ctx, fileOut, destinationDevice, sourceImage, geom, opts, log)
	if err != nil {
		_ = closeOut()
		return res, err
	}

	if err := finish(fileOut, destinationDevice, opts, log); err != nil {
		_ = closeOut()
		return res, err
	}

	return res, closeOut()
}

// writeTo runs the pipeline against an already-open destination.
func writeTo(ctx context.Context, fileOut *os.File, devName, sourceImage string, geom Geometry, opts Options, log *slog.Logger) (Result, error) {
	res := Result{Geometry: geom, ChunkSize: opts.ChunkSize, Writers: opts.Writers, Format: formatNone}

	client := opts.HTTPClient
	if client == nil {
		client = newHTTPClient()
	}

	// A separate cancellable layer so the stall watchdog can name itself as the
	// cause; errgroup derives its own context from this one.
	watchCtx, cancelWatch := context.WithCancelCause(ctx)
	defer cancelWatch(nil)

	resp, err := openSource(watchCtx, client, sourceImage)
	if err != nil {
		return res, err
	}
	defer resp.Body.Close()

	if err := checkFits(geom, opts, resp.ContentLength, devName); err != nil {
		return res, err
	}

	ctr := &counters{}
	g, gctx := errgroup.WithContext(watchCtx)

	raw := newReadAhead(gctx, g, resp.Body, opts.ReadAheadBufSize, opts.ReadAheadBufs, func(n int) {
		ctr.read.Add(int64(n))
	})

	dec := io.NopCloser(raw)
	if opts.Compressed {
		d, format, derr := findDecompressor(gctx, sourceImage, raw)
		if derr != nil {
			// Stop the read-ahead producer and let it finish before returning,
			// so no goroutine outlives this call.
			_ = raw.Close()
			_ = g.Wait()
			return res, derr
		}
		dec, res.Format = d, format
		if format == formatGzip || format == formatXZ {
			log.Info("this format decompresses on a single core and will likely be the bottleneck; "+
				"zstd or bzip2 images decode across all cores",
				"format", format)
		}
	}

	cw, mode := selectChunkWriter(fileOut, geom, opts, log)
	res.Mode = mode

	p := &pipeline{
		raw:     raw,
		dec:     dec,
		cw:      cw,
		pool:    newChunkPool(opts.ChunkSize),
		ctr:     ctr,
		chunk:   make(chan *Chunk, opts.QueueDepth),
		limit:   geom.SizeBytes,
		devName: devName,
	}

	log.Info("beginning write of image to disk",
		"image", filepath.Base(sourceImage), "disk", devName, "format", res.Format,
		"mode", mode, "chunkSize", humanize.IBytes(uint64(opts.ChunkSize)),
		"writers", opts.Writers, "queueDepth", opts.QueueDepth)

	stopWatchdog := startStallWatchdog(gctx, ctr, opts.StallTimeout, cancelWatch)
	stopProgress := startReporter(log, ctr, resp.ContentLength, mode == ModeDiff, opts.ProgressInterval)

	start := time.Now()
	runErr := p.run(gctx, g, opts.Writers)

	stopWatchdog()
	stopProgress()

	res.Duration = time.Since(start)
	res.BytesRead = ctr.read.Load()
	res.BytesDecoded = ctr.decoded.Load()
	res.BytesWritten = ctr.written.Load()
	res.BytesSkipped = ctr.skipped.Load()
	res.WriteCalls = ctr.calls.Load()
	if dw, ok := cw.(*diffWriter); ok {
		res.DiffFallbacks = dw.Fallbacks()
	}

	if runErr != nil {
		return res, runErr
	}

	return res, verifyComplete(res, resp.ContentLength, opts, sourceImage)
}

// checkFits fails early when the image is known to be larger than the device.
// Only an uncompressed image with a Content-Length has a knowable size up
// front; a compressed one is bounded by the chunker instead.
func checkFits(geom Geometry, opts Options, contentLength int64, devName string) error {
	if opts.Compressed || contentLength <= 0 || geom.SizeBytes <= 0 {
		return nil
	}
	if contentLength > geom.SizeBytes {
		return fmt.Errorf("%w: image is %s, %s is %s",
			ErrImageTooLarge, humanize.IBytes(uint64(contentLength)), devName, humanize.IBytes(uint64(geom.SizeBytes)))
	}
	return nil
}

// verifyComplete rejects a transfer that ended early.
//
// The previous implementation wrapped io.EOF inside its progress reader, which
// forced it to ignore io.EOF and io.ErrUnexpectedEOF from the copy, so a
// download cut short reported success and the machine booted a corrupt disk.
// Nothing swallows those errors now, and an uncompressed transfer is also
// checked against the advertised length.
func verifyComplete(res Result, contentLength int64, opts Options, sourceImage string) error {
	if res.BytesDecoded == 0 {
		return fmt.Errorf("%s produced no data", sourceImage)
	}
	if opts.Compressed || contentLength <= 0 {
		// A compressed stream's own trailer check (gzip CRC, zstd checksum)
		// already catches truncation, and it is no longer suppressed.
		return nil
	}
	if res.BytesRead != contentLength {
		return fmt.Errorf("truncated download of %s: got %d of %d bytes", sourceImage, res.BytesRead, contentLength)
	}
	return nil
}

// finish flushes the device and asks the kernel to re-read its partition table.
func finish(fileOut *os.File, devName string, opts Options, log *slog.Logger) error {
	if opts.SkipSync {
		return nil
	}

	if err := fileOut.Sync(); err != nil {
		return fmt.Errorf("syncing %s: %w", devName, err)
	}

	// The equivalent of partprobe. This fails with EINVAL when the destination
	// is a partition rather than a whole disk, which is not an error.
	if err := RereadPartitionTable(fileOut); err != nil {
		log.Info("error re-probing the partitions for the specified device", "err", err)
	}

	return nil
}

// WriteSimple preserves the call signature used before the pipeline rewrite.
//
// Deprecated: use [Write] with [Options].
func WriteSimple(ctx context.Context, log *slog.Logger, sourceImage, destinationDevice string, compressed bool, progressInterval time.Duration) error {
	_, err := Write(ctx, sourceImage, destinationDevice, Options{
		Logger:           log,
		ProgressInterval: progressInterval,
		Compressed:       compressed,
	})
	return err
}
