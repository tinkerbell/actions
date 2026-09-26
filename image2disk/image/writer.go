package image

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"sync"
	"sync/atomic"
)

// errMisalignedChunk reports a chunk offset that is not a multiple of the diff
// block size, which would silently misalign every comparison in the chunk.
var errMisalignedChunk = errors.New("chunk offset is not block aligned")

// WriteStats reports the outcome of a single WriteChunk call.
type WriteStats struct {
	// Written is the byte count actually sent to the device.
	Written int64
	// Skipped is the byte count found identical and not rewritten.
	Skipped int64
	// Calls is the number of pwrite syscalls issued.
	Calls int
}

// ChunkWriter writes one chunk of image data at an absolute device offset.
// Implementations must be safe for concurrent use by multiple goroutines.
type ChunkWriter interface {
	WriteChunk(off int64, data []byte) (WriteStats, error)
}

// device is the subset of *os.File the chunk writers need, so that tests can
// substitute an in-memory destination.
type device interface {
	io.WriterAt
	io.ReaderAt
}

// plainWriter writes every chunk unconditionally.
type plainWriter struct {
	dev io.WriterAt
}

// NewPlainWriter returns a ChunkWriter that writes every chunk unconditionally.
func NewPlainWriter(dev io.WriterAt) ChunkWriter {
	return &plainWriter{dev: dev}
}

// WriteChunk writes data at off.
func (w *plainWriter) WriteChunk(off int64, data []byte) (WriteStats, error) {
	// os.File.WriteAt already loops over pwrite until the buffer is consumed,
	// and handles EINTR, so no short-write loop is needed here.
	n, err := w.dev.WriteAt(data, off)
	st := WriteStats{Written: int64(n), Calls: 1}
	if err != nil {
		return st, fmt.Errorf("writing %d bytes at offset %d: %w", len(data), off, err)
	}
	return st, nil
}

// diffWriter reads the destination range before writing it and rewrites only
// the blocks whose contents differ.
//
// It is a win when most blocks already match, or on media that reads much
// faster than it writes. On a blank disk it is strictly twice the IO.
type diffWriter struct {
	dev       device
	blockSize int
	bufs      *bufPool

	fallbacks atomic.Int64
	logOnce   sync.Once
	log       *slog.Logger
}

// NewDiffWriter returns a ChunkWriter that reads the target range first and
// rewrites only the blockSize-aligned blocks whose contents differ. chunkSize
// sizes the internal compare buffers.
func NewDiffWriter(dev device, blockSize, chunkSize int, log *slog.Logger) ChunkWriter {
	return &diffWriter{
		dev:       dev,
		blockSize: blockSize,
		bufs:      newBufPool(chunkSize),
		log:       log,
	}
}

// Fallbacks reports how many chunks were written unconditionally because
// reading the device back failed.
func (w *diffWriter) Fallbacks() int64 {
	return w.fallbacks.Load()
}

// WriteChunk compares data against the device at off and writes only the runs
// of blocks that differ.
func (w *diffWriter) WriteChunk(off int64, data []byte) (WriteStats, error) {
	if off%int64(w.blockSize) != 0 {
		return WriteStats{}, fmt.Errorf("%w: offset %d, block size %d", errMisalignedChunk, off, w.blockSize)
	}

	bp := w.bufs.get()
	defer w.bufs.put(bp)
	cur := (*bp)[:len(data)]

	n, err := w.dev.ReadAt(cur, off)
	if err != nil && !errors.Is(err, io.EOF) {
		// A failed read must never be mistaken for "unchanged". Write it all.
		w.noteFallback(off, err)
		return w.writeRange(off, data, 0, len(data), WriteStats{})
	}
	// A short read means the device ends inside this chunk. ReadAt may also
	// report io.EOF alongside a full read, which min() folds into the same path.
	cmp := min(n, len(data))

	var (
		st       WriteStats
		runStart = -1
	)
	flush := func(end int) error {
		if runStart < 0 {
			return nil
		}
		next, ferr := w.writeRange(off, data, runStart, end, st)
		st = next
		runStart = -1
		return ferr
	}

	for i := 0; i < cmp; i += w.blockSize {
		end := min(i+w.blockSize, cmp)
		if bytes.Equal(data[i:end], cur[i:end]) {
			// Closing the run here is what coalesces consecutive differing
			// blocks into a single pwrite.
			if ferr := flush(i); ferr != nil {
				return st, ferr
			}
			st.Skipped += int64(end - i)
			continue
		}
		if runStart < 0 {
			runStart = i
		}
	}

	// Anything past the readable prefix has no counterpart on the device and so
	// must be written. Merging it into an open run costs no extra syscall.
	if cmp < len(data) && runStart < 0 {
		runStart = cmp
	}
	if ferr := flush(len(data)); ferr != nil {
		return st, ferr
	}
	return st, nil
}

// writeRange writes data[from:to] at its offset within the chunk and folds the
// result into st.
func (w *diffWriter) writeRange(off int64, data []byte, from, to int, st WriteStats) (WriteStats, error) {
	at := off + int64(from)
	n, err := w.dev.WriteAt(data[from:to], at)
	st.Written += int64(n)
	st.Calls++
	if err != nil {
		return st, fmt.Errorf("writing %d bytes at offset %d: %w", to-from, at, err)
	}
	return st, nil
}

// noteFallback records a read-back failure, logging only the first so that a
// systematically unreadable device does not produce one line per chunk.
func (w *diffWriter) noteFallback(off int64, err error) {
	w.fallbacks.Add(1)
	w.logOnce.Do(func() {
		w.log.Warn("reading the device back failed, writing this chunk unconditionally",
			"offset", off, "err", err)
	})
}

// selectChunkWriter returns the ChunkWriter for opts, downgrading ModeDiff to
// ModePlain when the destination cannot be read back. Probing once up front
// avoids paying for a full read pass that falls back on every chunk.
func selectChunkWriter(f *os.File, g Geometry, opts Options, log *slog.Logger) (ChunkWriter, WriteMode) {
	if opts.Mode != ModeDiff {
		return NewPlainWriter(f), ModePlain
	}

	probe := make([]byte, g.DiffBlockSize())
	if _, err := f.ReadAt(probe, 0); err != nil && !errors.Is(err, io.EOF) {
		log.Warn("destination cannot be read back, falling back to unconditional writes",
			"mode", ModeDiff, "err", err)
		return NewPlainWriter(f), ModePlain
	}

	return NewDiffWriter(f, opts.DiffBlockSize, opts.ChunkSize, log), ModeDiff
}
