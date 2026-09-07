// Package efipart finds a GPT partition by label and names its device node.
package efipart

import (
	"fmt"
	"path/filepath"
	"strconv"
	"strings"
	"unicode"

	diskfs "github.com/diskfs/go-diskfs"
	"github.com/diskfs/go-diskfs/partition/gpt"
)

// Partition is the location of a partition on a disk.
type Partition struct {
	// Index is the 1-based partition number.
	Index  int
	Offset int64
	Size   int64
}

// Locate finds the partition named label in the GPT of diskPath.
func Locate(diskPath, label string) (Partition, error) {
	d, err := diskfs.Open(diskPath, diskfs.WithOpenMode(diskfs.ReadOnly))
	if err != nil {
		return Partition{}, fmt.Errorf("opening %s: %w", diskPath, err)
	}
	defer d.Close()

	table, err := d.GetPartitionTable()
	if err != nil {
		return Partition{}, fmt.Errorf("reading partition table of %s: %w", diskPath, err)
	}

	gptTable, ok := table.(*gpt.Table)
	if !ok {
		return Partition{}, fmt.Errorf("%s has a %s partition table, expected GPT", diskPath, table.Type())
	}

	sectorSize := int64(gptTable.LogicalSectorSize)
	if sectorSize == 0 {
		sectorSize = d.LogicalBlocksize
	}

	for i, p := range gptTable.Partitions {
		if p == nil || p.Name != label {
			continue
		}

		index := p.Index
		if index == 0 {
			index = i + 1
		}

		return Partition{
			Index:  index,
			Offset: int64(p.Start) * sectorSize,
			Size:   int64(p.End-p.Start+1) * sectorSize,
		}, nil
	}

	return Partition{}, fmt.Errorf("no partition labelled %s found on %s", label, diskPath)
}

// Device returns the device node of partition index on disk, following the
// kernel's naming: /dev/sda1, /dev/nvme0n1p1, /dev/mmcblk0p2, and the
// -partN suffix used by /dev/disk/by-* symlinks.
func Device(disk string, index int) string {
	n := strconv.Itoa(index)

	for _, byDir := range []string{"/by-id/", "/by-path/", "/by-uuid/", "/by-partuuid/", "/by-partlabel/", "/by-diskseq/"} {
		if strings.Contains(disk, byDir) {
			return disk + "-part" + n
		}
	}

	base := filepath.Base(disk)
	if base != "" && unicode.IsDigit(rune(base[len(base)-1])) {
		return disk + "p" + n
	}

	return disk + n
}
