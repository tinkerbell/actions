package efipart

import (
	"path/filepath"
	"testing"

	diskfs "github.com/diskfs/go-diskfs"
	"github.com/diskfs/go-diskfs/partition/gpt"
)

func newDisk(t *testing.T, names ...string) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), "disk.img")
	d, err := diskfs.Create(path, 8*1024*1024, diskfs.SectorSize512)
	if err != nil {
		t.Fatal(err)
	}

	var parts []*gpt.Partition
	start := uint64(2048)
	for i, name := range names {
		parts = append(parts, &gpt.Partition{Index: i + 1, Start: start, End: start + 2047, Type: gpt.LinuxFilesystem, Name: name})
		start += 2048
	}
	if err := d.Partition(&gpt.Table{Partitions: parts, LogicalSectorSize: 512, PhysicalSectorSize: 512, ProtectiveMBR: true}); err != nil {
		t.Fatal(err)
	}
	if err := d.Close(); err != nil {
		t.Fatal(err)
	}

	return path
}

func TestLocateReturnsIndexAndOffset(t *testing.T) {
	path := newDisk(t, "EFI", "BIOS", "BOOT", "META")

	p, err := Locate(path, "BOOT")
	if err != nil {
		t.Fatalf("Locate() error: %v", err)
	}
	if p.Index != 3 || p.Offset != (2048+2*2048)*512 || p.Size != 2048*512 {
		t.Fatalf("unexpected partition: %+v", p)
	}
}

func TestLocateMissingLabel(t *testing.T) {
	path := newDisk(t, "BOOT")

	if _, err := Locate(path, "EFI"); err == nil {
		t.Fatal("expected an error for a missing label")
	}
}

func TestDevice(t *testing.T) {
	tests := []struct {
		disk  string
		index int
		want  string
	}{
		{"/dev/sda", 1, "/dev/sda1"},
		{"/dev/vdb", 3, "/dev/vdb3"},
		{"/dev/nvme0n1", 1, "/dev/nvme0n1p1"},
		{"/dev/mmcblk0", 2, "/dev/mmcblk0p2"},
		{"/dev/loop0", 1, "/dev/loop0p1"},
		{"/dev/disk/by-id/nvme-Foo_123", 1, "/dev/disk/by-id/nvme-Foo_123-part1"},
		{"/dev/disk/by-path/pci-0000:00:1f.2-ata-1", 2, "/dev/disk/by-path/pci-0000:00:1f.2-ata-1-part2"},
	}
	for _, tt := range tests {
		t.Run(tt.disk, func(t *testing.T) {
			if got := Device(tt.disk, tt.index); got != tt.want {
				t.Fatalf("Device(%q, %d) = %q, want %q", tt.disk, tt.index, got, tt.want)
			}
		})
	}
}
