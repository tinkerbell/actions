package image

import (
	"context"
	"errors"
	"fmt"
	"io"

	"golang.org/x/sync/errgroup"
)

// pipeline streams decoded image data onto a device through four stages:
// an HTTP read-ahead ring, a decompressor, a chunker that emits aligned
// offset-tagged chunks, and a pool of writer workers issuing pwrite.
//
// Backpressure comes entirely from the three bounded queues; the slowest stage
// throttles everything upstream and no buffer grows without bound.
type pipeline struct {
	raw   *readAheadReader
	dec   io.ReadCloser
	cw    ChunkWriter
	pool  *chunkPool
	ctr   *counters
	chunk chan *Chunk

	// limit is the destination size in bytes, or zero when unknown.
	limit   int64
	devName string
}

// chunker reads the decoded stream and emits aligned, offset-tagged chunks.
// It is the sole producer on p.chunk and closes it on return.
func (p *pipeline) chunker(ctx context.Context, src io.Reader) (err error) {
	// Defers run last-in-first-out, so the order here is: close the decoder
	// (joining any goroutines it spawned, so none is left mid-Read), then stop
	// the read-ahead producer, then release the writer workers. Reversing this
	// leaves a decoder worker reading from a closed ring.
	defer close(p.chunk)
	defer func() { err = errors.Join(err, p.raw.Close()) }()
	defer func() { err = errors.Join(err, p.dec.Close()) }()

	var off int64
	for {
		c := p.pool.get()
		c.Data = c.Data[:p.pool.size]

		// io.ReadFull, not Read: it guarantees every chunk but the last is
		// exactly chunkSize, so every offset is block aligned. A bare Read
		// returns whatever the decompressor felt like and would misalign the
		// diff writer's comparisons.
		n, rerr := io.ReadFull(src, c.Data)

		if n == 0 {
			p.pool.put(c)
			if rerr == nil || errors.Is(rerr, io.EOF) {
				return nil
			}
			return fmt.Errorf("decoding image at offset %d: %w", off, rerr)
		}
		if rerr != nil && !errors.Is(rerr, io.EOF) && !errors.Is(rerr, io.ErrUnexpectedEOF) {
			p.pool.put(c)
			return fmt.Errorf("decoding image at offset %d: %w", off, rerr)
		}

		c.Data, c.Off = c.Data[:n], off
		off += int64(n)
		p.ctr.decoded.Add(int64(n))

		if p.limit > 0 && off > p.limit {
			p.pool.put(c)
			return fmt.Errorf("%w: image reached %d bytes, %s is %d bytes",
				ErrImageTooLarge, off, p.devName, p.limit)
		}

		select {
		case p.chunk <- c:
		case <-ctx.Done():
			p.pool.put(c)
			return context.Cause(ctx)
		}

		// A short read is the end of the stream; the tail chunk is already queued.
		if n < p.pool.size {
			return nil
		}
	}
}

// writeWorker consumes chunks and writes them to the device. Writes use pwrite
// at explicit offsets, so ordering between workers does not matter.
func (p *pipeline) writeWorker(ctx context.Context) error {
	for {
		var (
			c  *Chunk
			ok bool
		)
		select {
		case c, ok = <-p.chunk:
			if !ok {
				return nil
			}
		case <-ctx.Done():
			return context.Cause(ctx)
		}

		// Capture these before returning the chunk to the pool, or the error
		// message below describes a recycled buffer.
		off, n := c.Off, len(c.Data)

		st, err := p.cw.WriteChunk(off, c.Data)
		p.pool.put(c)

		p.ctr.written.Add(st.Written)
		p.ctr.skipped.Add(st.Skipped)
		p.ctr.calls.Add(int64(st.Calls))

		if err != nil {
			return fmt.Errorf("writing %d bytes at offset %d to %s: %w", n, off, p.devName, err)
		}
	}
}

// run starts every stage and waits for them all. The first error wins and
// cancels the rest.
func (p *pipeline) run(ctx context.Context, g *errgroup.Group, writers int) error {
	for range writers {
		g.Go(func() error { return p.writeWorker(ctx) })
	}
	g.Go(func() error { return p.chunker(ctx, p.dec) })

	return g.Wait()
}
