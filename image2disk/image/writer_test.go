package image

import (
	"bytes"
	"errors"
	"io"
	"log/slog"
	"math/rand"
	"os"
	"path/filepath"
	"sync"
	"testing"
)

const testBlockSize = 4096

// memDevice is an in-memory destination that mimics the parts of *os.File the
// chunk writers rely on, including returning a bare io.EOF on a short ReadAt.
type memDevice struct {
	mu      sync.Mutex
	data    []byte
	writes  int
	readErr error
}

func newMemDevice(size int) *memDevice {
	return &memDevice{data: make([]byte, size)}
}

func (d *memDevice) ReadAt(p []byte, off int64) (int, error) {
	d.mu.Lock()
	defer d.mu.Unlock()

	if d.readErr != nil {
		return 0, d.readErr
	}
	if off >= int64(len(d.data)) {
		return 0, io.EOF
	}
	n := copy(p, d.data[off:])
	if n < len(p) {
		// os.File.ReadAt returns a bare io.EOF here; a fake that wraps it would
		// pass tests the real thing fails.
		return n, io.EOF
	}
	return n, nil
}

func (d *memDevice) WriteAt(p []byte, off int64) (int, error) {
	d.mu.Lock()
	defer d.mu.Unlock()

	d.writes++
	if end := int(off) + len(p); end > len(d.data) {
		d.data = append(d.data, make([]byte, end-len(d.data))...)
	}
	return copy(d.data[off:], p), nil
}

func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

// randomBlocks returns n blocks of deterministic pseudo-random data.
func randomBlocks(n int) []byte {
	b := make([]byte, n*testBlockSize)
	r := rand.New(rand.NewSource(int64(n)))
	_, _ = r.Read(b)
	return b
}

func TestPlainWriterWritesEverything(t *testing.T) {
	data := randomBlocks(4)
	dev := newMemDevice(len(data))

	st, err := NewPlainWriter(dev).WriteChunk(0, data)
	if err != nil {
		t.Fatalf("WriteChunk: %v", err)
	}
	if st.Written != int64(len(data)) || st.Skipped != 0 || st.Calls != 1 {
		t.Errorf("stats = %+v, want written=%d skipped=0 calls=1", st, len(data))
	}
	if !bytes.Equal(dev.data, data) {
		t.Error("device contents do not match the written data")
	}
}

func TestDiffWriterSkipsIdenticalChunk(t *testing.T) {
	data := randomBlocks(16)
	dev := newMemDevice(len(data))
	copy(dev.data, data)

	w := NewDiffWriter(dev, testBlockSize, len(data), testLogger())
	st, err := w.WriteChunk(0, data)
	if err != nil {
		t.Fatalf("WriteChunk: %v", err)
	}

	if st.Written != 0 {
		t.Errorf("written = %d, want 0", st.Written)
	}
	if st.Skipped != int64(len(data)) {
		t.Errorf("skipped = %d, want %d", st.Skipped, len(data))
	}
	if st.Calls != 0 || dev.writes != 0 {
		t.Errorf("issued %d WriteAt calls, want 0", dev.writes)
	}
}

func TestDiffWriterCoalescesRuns(t *testing.T) {
	data := randomBlocks(16)
	dev := newMemDevice(len(data))
	copy(dev.data, data)

	// Blocks 2 and 3 are adjacent and must coalesce into one write; block 10
	// stands alone. Three changed blocks, two syscalls.
	for _, blk := range []int{2, 3, 10} {
		for i := range testBlockSize {
			dev.data[blk*testBlockSize+i] ^= 0xff
		}
	}

	w := NewDiffWriter(dev, testBlockSize, len(data), testLogger())
	st, err := w.WriteChunk(0, data)
	if err != nil {
		t.Fatalf("WriteChunk: %v", err)
	}

	if want := int64(3 * testBlockSize); st.Written != want {
		t.Errorf("written = %d, want %d", st.Written, want)
	}
	if want := int64(13 * testBlockSize); st.Skipped != want {
		t.Errorf("skipped = %d, want %d", st.Skipped, want)
	}
	if st.Calls != 2 {
		t.Errorf("calls = %d, want 2 (blocks 2-3 coalesced, block 10 separate)", st.Calls)
	}
	if !bytes.Equal(dev.data, data) {
		t.Error("device contents do not match after the diff write")
	}
}

func TestDiffWriterUnalignedTail(t *testing.T) {
	// Two whole blocks plus a partial one, as the final chunk of a stream.
	data := append(randomBlocks(2), bytes.Repeat([]byte{0xab}, 100)...)
	dev := newMemDevice(len(data))
	copy(dev.data, data)

	w := NewDiffWriter(dev, testBlockSize, len(data), testLogger())
	st, err := w.WriteChunk(0, data)
	if err != nil {
		t.Fatalf("WriteChunk: %v", err)
	}
	if st.Written != 0 || st.Skipped != int64(len(data)) {
		t.Errorf("stats = %+v, want the whole unaligned chunk skipped (%d bytes)", st, len(data))
	}
}

func TestDiffWriterShortDeviceWritesTail(t *testing.T) {
	data := randomBlocks(8)
	// The device holds only the first six blocks; the rest has no counterpart
	// and must be written blind.
	dev := newMemDevice(6 * testBlockSize)
	copy(dev.data, data)

	w := NewDiffWriter(dev, testBlockSize, len(data), testLogger())
	st, err := w.WriteChunk(0, data)
	if err != nil {
		t.Fatalf("WriteChunk: %v", err)
	}

	if want := int64(2 * testBlockSize); st.Written != want {
		t.Errorf("written = %d, want %d (the two blocks past the device end)", st.Written, want)
	}
	if want := int64(6 * testBlockSize); st.Skipped != want {
		t.Errorf("skipped = %d, want %d", st.Skipped, want)
	}
	if !bytes.Equal(dev.data, data) {
		t.Error("device contents do not match after writing past the old end")
	}
}

func TestDiffWriterFallsBackWhenReadFails(t *testing.T) {
	data := randomBlocks(4)
	dev := newMemDevice(len(data))
	copy(dev.data, data)
	dev.readErr = errors.New("simulated media error")

	w := NewDiffWriter(dev, testBlockSize, len(data), testLogger())
	st, err := w.WriteChunk(0, data)
	if err != nil {
		t.Fatalf("WriteChunk: %v", err)
	}

	// A failed read must never be interpreted as "unchanged".
	if st.Written != int64(len(data)) || st.Skipped != 0 {
		t.Errorf("stats = %+v, want the whole chunk written unconditionally", st)
	}
	dw, ok := w.(*diffWriter)
	if !ok {
		t.Fatalf("NewDiffWriter returned %T", w)
	}
	if dw.Fallbacks() != 1 {
		t.Errorf("fallbacks = %d, want 1", dw.Fallbacks())
	}
}

func TestDiffWriterRejectsMisalignedOffset(t *testing.T) {
	data := randomBlocks(2)
	dev := newMemDevice(len(data) + 512)

	w := NewDiffWriter(dev, testBlockSize, len(data), testLogger())
	if _, err := w.WriteChunk(512, data); !errors.Is(err, errMisalignedChunk) {
		t.Errorf("err = %v, want errMisalignedChunk", err)
	}
}

func TestDiffWriterAgainstRealFile(t *testing.T) {
	// The same round trip against a real *os.File, which is what production
	// uses and what defines the ReadAt short-read semantics.
	path := filepath.Join(t.TempDir(), "disk.img")
	data := randomBlocks(32)

	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	f, err := os.OpenFile(path, os.O_RDWR, 0o600)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	w := NewDiffWriter(f, testBlockSize, len(data), testLogger())

	st, err := w.WriteChunk(0, data)
	if err != nil {
		t.Fatalf("WriteChunk: %v", err)
	}
	if st.Written != 0 || st.Skipped != int64(len(data)) {
		t.Errorf("rewriting identical data: stats = %+v, want everything skipped", st)
	}

	// Now change one block and confirm only it is rewritten.
	changed := bytes.Clone(data)
	for i := range testBlockSize {
		changed[7*testBlockSize+i] = 0x5a
	}
	st, err = w.WriteChunk(0, changed)
	if err != nil {
		t.Fatalf("WriteChunk: %v", err)
	}
	if st.Written != testBlockSize || st.Calls != 1 {
		t.Errorf("stats = %+v, want exactly one block written in one call", st)
	}

	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, changed) {
		t.Error("file contents do not match after the diff write")
	}
}

func TestSelectChunkWriterDowngradesUnreadableDevice(t *testing.T) {
	// A write-only file cannot be read back, so diff mode must downgrade rather
	// than fall back on every single chunk.
	path := filepath.Join(t.TempDir(), "disk.img")
	if err := os.WriteFile(path, randomBlocks(2), 0o600); err != nil {
		t.Fatal(err)
	}
	f, err := os.OpenFile(path, os.O_WRONLY, 0o600)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	g := Geometry{}.normalize()
	opts := Options{Mode: ModeDiff, DiffBlockSize: testBlockSize, ChunkSize: testBlockSize}
	_, mode := selectChunkWriter(f, g, opts, testLogger())
	if mode != ModePlain {
		t.Errorf("mode = %q, want %q", mode, ModePlain)
	}
}
