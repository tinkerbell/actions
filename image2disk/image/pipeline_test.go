package image

import (
	"bytes"
	"context"
	"errors"
	"io"
	"testing"
	"time"

	"golang.org/x/sync/errgroup"
)

// recordingWriter captures every chunk it is handed.
type recordingWriter struct {
	memDevice
	fail error
}

func (w *recordingWriter) WriteChunk(off int64, data []byte) (WriteStats, error) {
	if w.fail != nil {
		return WriteStats{}, w.fail
	}
	n, err := w.WriteAt(data, off)
	return WriteStats{Written: int64(n), Calls: 1}, err
}

// runPipeline drives the pipeline over src and returns the destination contents.
func runPipeline(t *testing.T, src []byte, chunkSize, writers, queueDepth int, cw ChunkWriter) error {
	t.Helper()

	g, gctx := errgroup.WithContext(context.Background())
	raw := newReadAhead(gctx, g, bytes.NewReader(src), 8192, 4, func(int) {})

	p := &pipeline{
		raw:     raw,
		dec:     io.NopCloser(raw),
		cw:      cw,
		pool:    newChunkPool(chunkSize),
		ctr:     &counters{},
		chunk:   make(chan *Chunk, queueDepth),
		devName: "test",
	}

	return p.run(gctx, g, writers)
}

func TestPipelineWritesExactBytes(t *testing.T) {
	for _, tc := range []struct {
		name string
		size int
	}{
		{"exact multiple of chunk size", 8 * 1024},
		{"short tail", 8*1024 + 333},
		{"smaller than one chunk", 500},
		{"single byte", 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			src := make([]byte, tc.size)
			for i := range src {
				src[i] = byte(i)
			}

			dev := &recordingWriter{}
			// A generous timeout: the real failure this guards against is the
			// pipeline hanging on the success path, not being slow.
			done := make(chan error, 1)
			go func() { done <- runPipeline(t, src, 1024, 3, 2, dev) }()

			select {
			case err := <-done:
				if err != nil {
					t.Fatalf("run: %v", err)
				}
			case <-time.After(30 * time.Second):
				t.Fatal("pipeline did not finish: the success-path shutdown deadlocked")
			}

			if !bytes.Equal(dev.data, src) {
				t.Errorf("destination has %d bytes, want %d matching the source", len(dev.data), len(src))
			}
		})
	}
}

func TestPipelineChunkOffsetsAreAligned(t *testing.T) {
	const chunkSize = 1024
	src := make([]byte, chunkSize*5+17)

	var offsets []int64
	cw := chunkWriterFunc(func(off int64, data []byte) (WriteStats, error) {
		offsets = append(offsets, off)
		return WriteStats{Written: int64(len(data)), Calls: 1}, nil
	})

	// One writer keeps the recorded order deterministic.
	if err := runPipeline(t, src, chunkSize, 1, 1, cw); err != nil {
		t.Fatalf("run: %v", err)
	}

	if len(offsets) != 6 {
		t.Fatalf("got %d chunks, want 6", len(offsets))
	}
	for i, off := range offsets {
		if want := int64(i * chunkSize); off != want {
			t.Errorf("chunk %d offset = %d, want %d", i, off, want)
		}
	}
}

func TestPipelinePropagatesWriterError(t *testing.T) {
	sentinel := errors.New("disk on fire")
	src := make([]byte, 1024*64)

	done := make(chan error, 1)
	go func() { done <- runPipeline(t, src, 1024, 2, 2, &recordingWriter{fail: sentinel}) }()

	select {
	case err := <-done:
		if !errors.Is(err, sentinel) {
			t.Errorf("err = %v, want it to wrap %v", err, sentinel)
		}
	case <-time.After(30 * time.Second):
		t.Fatal("a failing writer deadlocked the pipeline instead of cancelling it")
	}
}

func TestPipelineEnforcesDeviceSize(t *testing.T) {
	src := make([]byte, 8192)

	g, gctx := errgroup.WithContext(context.Background())
	raw := newReadAhead(gctx, g, bytes.NewReader(src), 4096, 2, func(int) {})
	p := &pipeline{
		raw:     raw,
		dec:     io.NopCloser(raw),
		cw:      &recordingWriter{},
		pool:    newChunkPool(1024),
		ctr:     &counters{},
		chunk:   make(chan *Chunk, 2),
		limit:   4096, // the image is twice this
		devName: "test",
	}

	if err := p.run(gctx, g, 2); !errors.Is(err, ErrImageTooLarge) {
		t.Errorf("err = %v, want ErrImageTooLarge", err)
	}
}

func TestReadAheadReturnsBareEOF(t *testing.T) {
	// io.Copy compares against io.EOF by identity and decompressors treat a
	// wrapped EOF as corruption, so this must never be wrapped.
	g, gctx := errgroup.WithContext(context.Background())
	raw := newReadAhead(gctx, g, bytes.NewReader([]byte("hello")), 8, 2, func(int) {})

	got, err := io.ReadAll(raw)
	if err != nil {
		t.Fatalf("ReadAll: %v", err)
	}
	if string(got) != "hello" {
		t.Errorf("got %q, want %q", got, "hello")
	}

	if _, err := raw.Read(make([]byte, 1)); err != io.EOF { //nolint:errorlint,err113 // identity is exactly what is under test.
		t.Errorf("err = %v (%T), want the io.EOF sentinel itself", err, err)
	}

	_ = raw.Close()
	if err := g.Wait(); err != nil {
		t.Errorf("group: %v", err)
	}
}

func TestReadAheadSurfacesSourceError(t *testing.T) {
	sentinel := errors.New("connection reset")
	g, gctx := errgroup.WithContext(context.Background())
	raw := newReadAhead(gctx, g, io.MultiReader(bytes.NewReader([]byte("abc")), errReader{sentinel}), 8, 2, func(int) {})

	if _, err := io.ReadAll(raw); !errors.Is(err, sentinel) {
		t.Errorf("err = %v, want it to wrap %v", err, sentinel)
	}
}

// errReader fails on every read.
type errReader struct{ err error }

func (r errReader) Read([]byte) (int, error) { return 0, r.err }

// chunkWriterFunc adapts a function to the ChunkWriter interface.
type chunkWriterFunc func(off int64, data []byte) (WriteStats, error)

func (f chunkWriterFunc) WriteChunk(off int64, data []byte) (WriteStats, error) {
	return f(off, data)
}
