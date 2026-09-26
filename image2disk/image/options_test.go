package image

import (
	"os"
	"path/filepath"
	"testing"
)

// footprint is the pipeline's peak buffer usage for these options.
func footprint(o Options) int64 {
	live := int64(o.QueueDepth + o.Writers + 1)
	if o.Mode == ModeDiff {
		live += int64(o.Writers)
	}
	return int64(o.ReadAheadBufs)*int64(o.ReadAheadBufSize) + live*int64(o.ChunkSize)
}

func TestApplyDefaultsRespectsBudget(t *testing.T) {
	g := Geometry{}.normalize()

	for _, tc := range []struct {
		name   string
		budget int64
		mode   WriteMode
	}{
		{"one gigabyte host", 768 << 20, ModePlain},
		{"one gigabyte host, diff", 768 << 20, ModeDiff},
		{"modest ramdisk", 64 << 20, ModePlain},
		{"modest ramdisk, diff", 64 << 20, ModeDiff},
		{"very tight", 8 << 20, ModeDiff},
	} {
		t.Run(tc.name, func(t *testing.T) {
			o := Options{MemoryBudget: tc.budget, Mode: tc.mode}
			o.applyDefaults(g)

			if o.ChunkSize <= 0 || o.ChunkSize%g.Align() != 0 {
				t.Errorf("chunk size %d is not a positive multiple of the %d-byte alignment", o.ChunkSize, g.Align())
			}
			if o.Writers < 1 || o.QueueDepth < 1 {
				t.Errorf("writers=%d queueDepth=%d, both must be at least 1", o.Writers, o.QueueDepth)
			}
			if got := footprint(o); got > tc.budget {
				t.Errorf("footprint %d exceeds budget %d (chunk=%d queue=%d writers=%d readahead=%dx%d)",
					got, tc.budget, o.ChunkSize, o.QueueDepth, o.Writers, o.ReadAheadBufs, o.ReadAheadBufSize)
			}
		})
	}
}

func TestApplyDefaultsRotationalUsesOneWriter(t *testing.T) {
	o := Options{}
	o.applyDefaults(Geometry{Rotational: true}.normalize())

	// Concurrent pwrites defeat seek ordering on spinning media.
	if o.Writers != 1 {
		t.Errorf("writers = %d, want 1 on rotational media", o.Writers)
	}
}

func TestApplyDefaultsHonoursExplicitChunkSize(t *testing.T) {
	o := Options{ChunkSize: 1 << 20}
	o.applyDefaults(Geometry{}.normalize())

	if o.ChunkSize != 1<<20 {
		t.Errorf("chunk size = %d, want the requested 1 MiB", o.ChunkSize)
	}
}

func TestProbeGeometryOnRegularFile(t *testing.T) {
	// Every BLK* ioctl answers ENOTTY on a regular file. That is the fallback
	// path, not an error, and it is what makes the writers testable as root.
	path := filepath.Join(t.TempDir(), "disk.img")
	if err := os.WriteFile(path, make([]byte, 8192), 0o600); err != nil {
		t.Fatal(err)
	}
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	g, err := ProbeGeometry(f)
	if err != nil {
		t.Fatalf("ProbeGeometry: %v", err)
	}
	if g.IsBlockDevice {
		t.Error("a regular file was reported as a block device")
	}
	if g.SizeBytes != 8192 {
		t.Errorf("size = %d, want 8192", g.SizeBytes)
	}
	if g.Align() < defaultBlockSize {
		t.Errorf("alignment = %d, want at least %d", g.Align(), defaultBlockSize)
	}
}

func TestAlignChunkSizeClamps(t *testing.T) {
	g := Geometry{}.normalize()

	if got := g.AlignChunkSize(1); got != g.Align() {
		t.Errorf("AlignChunkSize(1) = %d, want one alignment unit (%d)", got, g.Align())
	}
	if got := g.AlignChunkSize(1 << 30); got != maxChunkSize {
		t.Errorf("AlignChunkSize(1GiB) = %d, want the %d cap", got, maxChunkSize)
	}
	if got := g.AlignChunkSize(defaultBlockSize*3 + 17); got != defaultBlockSize*3 {
		t.Errorf("AlignChunkSize rounded to %d, want %d", got, defaultBlockSize*3)
	}
}
