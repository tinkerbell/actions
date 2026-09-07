package meta

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"testing"

	diskfs "github.com/diskfs/go-diskfs"
	"github.com/diskfs/go-diskfs/partition/gpt"
	advtalos "github.com/siderolabs/go-adv/adv/talos"
)

const sectorSize = 512

// newTalosLikeDisk creates a GPT image with an EFI partition followed by a
// 1 MiB META partition, like a Talos metal image.
func newTalosLikeDisk(t *testing.T, withMeta bool) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), "disk.img")
	d, err := diskfs.Create(path, 8*1024*1024, diskfs.SectorSize512)
	if err != nil {
		t.Fatal(err)
	}

	parts := []*gpt.Partition{
		{Index: 1, Start: 2048, End: 4095, Type: gpt.EFISystemPartition, Name: "EFI"},
	}
	if withMeta {
		parts = append(parts, &gpt.Partition{Index: 2, Start: 4096, End: 6143, Type: gpt.LinuxFilesystem, Name: "META"})
	}
	if err := d.Partition(&gpt.Table{Partitions: parts, LogicalSectorSize: sectorSize, PhysicalSectorSize: sectorSize, ProtectiveMBR: true}); err != nil {
		t.Fatal(err)
	}
	if err := d.Close(); err != nil {
		t.Fatal(err)
	}

	return path
}

func readTalosADV(t *testing.T, path string, offset int64) *advtalos.ADV {
	t.Helper()

	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	adv, err := advtalos.NewADV(io.NewSectionReader(f, offset, advtalos.Size))
	if err != nil {
		t.Fatalf("loading ADV: %v", err)
	}

	return adv
}

func TestLocateFindsMetaPartition(t *testing.T) {
	path := newTalosLikeDisk(t, true)

	p, err := Locate(path)
	if err != nil {
		t.Fatalf("Locate() error: %v", err)
	}
	if p.Offset != 4096*sectorSize {
		t.Fatalf("expected offset %d, got %d", 4096*sectorSize, p.Offset)
	}
	if p.Size != 2048*sectorSize {
		t.Fatalf("expected size %d, got %d", 2048*sectorSize, p.Size)
	}
}

func TestLocateFailsWithoutMetaPartition(t *testing.T) {
	path := newTalosLikeDisk(t, false)

	if _, err := Locate(path); err == nil {
		t.Fatal("expected an error when META is missing")
	}
}

func TestWriteTagOnBlankPartition(t *testing.T) {
	path := newTalosLikeDisk(t, true)
	value := []byte("hostnames:\n    - hostname: node1\n      layer: platform\n")

	if err := WriteTag(path, MetalNetworkPlatformConfig, value); err != nil {
		t.Fatalf("WriteTag() error: %v", err)
	}

	adv := readTalosADV(t, path, 4096*sectorSize)
	got, ok := adv.ReadTagBytes(MetalNetworkPlatformConfig)
	if !ok || !bytes.Equal(got, value) {
		t.Fatalf("expected tag 0xa to hold the document, got ok=%v value=%q", ok, got)
	}

	// Talos keeps two copies of the ADV back to back.
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	first := make([]byte, advtalos.Length)
	second := make([]byte, advtalos.Length)
	if _, err := f.ReadAt(first, 4096*sectorSize); err != nil {
		t.Fatal(err)
	}
	if _, err := f.ReadAt(second, 4096*sectorSize+advtalos.Length); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(first, second) {
		t.Fatal("expected the second ADV copy to match the first")
	}
}

func TestWriteTagPreservesOtherTags(t *testing.T) {
	path := newTalosLikeDisk(t, true)

	if err := WriteTag(path, 0x0c, []byte("keep me")); err != nil {
		t.Fatalf("WriteTag() error: %v", err)
	}
	if err := WriteTag(path, MetalNetworkPlatformConfig, []byte("first")); err != nil {
		t.Fatalf("WriteTag() error: %v", err)
	}
	if err := WriteTag(path, MetalNetworkPlatformConfig, []byte("second")); err != nil {
		t.Fatalf("WriteTag() error: %v", err)
	}

	adv := readTalosADV(t, path, 4096*sectorSize)
	if got, ok := adv.ReadTag(MetalNetworkPlatformConfig); !ok || got != "second" {
		t.Fatalf("expected tag 0xa to be overwritten, got ok=%v value=%q", ok, got)
	}
	if got, ok := adv.ReadTag(0x0c); !ok || got != "keep me" {
		t.Fatalf("expected tag 0xc to survive, got ok=%v value=%q", ok, got)
	}
}

func TestReadTag(t *testing.T) {
	path := newTalosLikeDisk(t, true)

	if _, ok, err := ReadTag(path, MetalNetworkPlatformConfig); err != nil || ok {
		t.Fatalf("expected no tag on a blank partition, got ok=%v err=%v", ok, err)
	}
	if err := WriteTag(path, MetalNetworkPlatformConfig, []byte("doc")); err != nil {
		t.Fatalf("WriteTag() error: %v", err)
	}
	got, ok, err := ReadTag(path, MetalNetworkPlatformConfig)
	if err != nil || !ok || string(got) != "doc" {
		t.Fatalf("expected doc, got ok=%v value=%q err=%v", ok, got, err)
	}
}

func TestWriteTagLeavesSyslinuxAreaValid(t *testing.T) {
	path := newTalosLikeDisk(t, true)

	if err := WriteTag(path, MetalNetworkPlatformConfig, []byte("doc")); err != nil {
		t.Fatalf("WriteTag() error: %v", err)
	}

	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	// The legacy syslinux ADV occupies the last 1 KiB of the partition; Talos
	// writes it back untouched, so a blank partition stays blank there.
	tail := make([]byte, 1024)
	if _, err := f.ReadAt(tail, 6144*sectorSize-1024); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(tail, make([]byte, 1024)) {
		t.Fatal("expected the syslinux ADV area to remain zeroed")
	}
}
