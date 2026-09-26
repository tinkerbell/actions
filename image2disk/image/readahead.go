package image

import (
	"context"
	"errors"
	"fmt"
	"io"
	"sync"

	"golang.org/x/sync/errgroup"
)

// readAheadReader decouples a bursty source from its consumer by pre-reading
// into a bounded ring of buffers on a dedicated goroutine.
//
// Its Read returns a bare io.EOF at end of stream and never wraps it: io.Copy
// compares against io.EOF by identity, and every decompressor treats a wrapped
// EOF as a corrupt stream rather than a clean end.
type readAheadReader struct {
	bufs chan []byte   // filled buffers, in order; closed by the producer
	free chan []byte   // recycled buffers
	done chan struct{} // closed by Close to stop the producer
	once sync.Once

	// cur is the unconsumed tail of the buffer being read; curBase is that same
	// buffer at full capacity, kept so it can be recycled. Reslicing cur moves
	// its base pointer, so cap(cur) is not the original capacity.
	cur     []byte
	curBase []byte

	// err is the producer's terminal error. It is written before bufs is closed
	// and read only after a receive on the closed bufs, so the channel close
	// provides the happens-before edge and no mutex is needed.
	err error
}

// newReadAhead starts a producer goroutine in g that pre-reads from src into a
// ring of nbufs buffers of bufSize bytes each. onRead is called with the byte
// count of every successful read.
func newReadAhead(ctx context.Context, g *errgroup.Group, src io.Reader, bufSize, nbufs int, onRead func(int)) *readAheadReader {
	r := &readAheadReader{
		bufs: make(chan []byte, nbufs),
		free: make(chan []byte, nbufs),
		done: make(chan struct{}),
	}
	for range nbufs {
		r.free <- make([]byte, 0, bufSize)
	}

	g.Go(func() error { return r.produce(ctx, src, bufSize, onRead) })

	return r
}

// produce fills buffers from src until the source ends, Close is called, or ctx
// is cancelled.
func (r *readAheadReader) produce(ctx context.Context, src io.Reader, bufSize int, onRead func(int)) error {
	for {
		var buf []byte
		select {
		case buf = <-r.free:
		case <-r.done:
			return nil
		case <-ctx.Done():
			return context.Cause(ctx)
		}

		n, err := io.ReadFull(src, buf[:bufSize])
		if n > 0 {
			onRead(n)
			select {
			case r.bufs <- buf[:n]:
			case <-r.done:
				return nil
			case <-ctx.Done():
				return context.Cause(ctx)
			}
		}

		switch {
		case err == nil:
			continue
		case errors.Is(err, io.EOF), errors.Is(err, io.ErrUnexpectedEOF):
			close(r.bufs)
			return nil
		default:
			r.err = fmt.Errorf("reading image source: %w", err)
			close(r.bufs)
			return r.err
		}
	}
}

// Read returns pre-read bytes, blocking only when the producer has not caught up.
func (r *readAheadReader) Read(p []byte) (int, error) {
	for len(r.cur) == 0 {
		if r.curBase != nil {
			// free has capacity for every buffer in the ring, so this can never
			// block; the default arm exists only to make that impossible to
			// turn into a deadlock.
			select {
			case r.free <- r.curBase:
			default:
			}
			r.curBase = nil
		}

		b, ok := <-r.bufs
		if !ok {
			if r.err != nil {
				return 0, r.err
			}
			return 0, io.EOF
		}
		r.cur = b
		r.curBase = b[:0:cap(b)]
	}

	n := copy(p, r.cur)
	r.cur = r.cur[n:]

	return n, nil
}

// Close stops the producer. It is idempotent, and does not close the underlying
// reader.
//
// Calling it is what prevents the success-path deadlock: the chunker can finish
// while the producer is still blocked sending a buffer it read ahead past the
// end of the compressed stream. Nothing has failed, so the group context is
// never cancelled, and without done the producer would block forever.
func (r *readAheadReader) Close() error {
	r.once.Do(func() { close(r.done) })
	return nil
}
