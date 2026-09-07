// Package meta reads and writes tags in the Talos META partition.
//
// The on-disk layout is the one Talos' internal meta package uses: the Talos
// ADV (two redundant 256 KiB copies) at the start of the partition and the
// legacy syslinux ADV in the last 1 KiB. Both are handled by the
// github.com/siderolabs/go-adv module that Talos itself uses.
package meta

import (
	"errors"
	"fmt"
	"io"
	"os"

	diskfs "github.com/diskfs/go-diskfs"
	"github.com/diskfs/go-diskfs/partition/gpt"
	"github.com/siderolabs/go-adv/adv"
	"github.com/siderolabs/go-adv/adv/syslinux"
	advtalos "github.com/siderolabs/go-adv/adv/talos"
)

// MetalNetworkPlatformConfig is the META key holding the metal platform
// network configuration; it matches meta.MetalNetworkPlatformConfig in
// github.com/siderolabs/talos/pkg/machinery/meta.
const MetalNetworkPlatformConfig uint8 = 0x0a

// PartitionLabel is the GPT partition name Talos gives the META partition.
const PartitionLabel = "META"

// Partition is the location of the META partition on a disk.
type Partition struct {
	Path   string
	Offset int64
	Size   int64
}

// Locate finds the META partition in the GPT of diskPath.
func Locate(diskPath string) (Partition, error) {
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
		return Partition{}, fmt.Errorf("%s has a %s partition table, Talos requires GPT", diskPath, table.Type())
	}

	sectorSize := int64(gptTable.LogicalSectorSize)
	if sectorSize == 0 {
		sectorSize = d.LogicalBlocksize
	}

	for _, p := range gptTable.Partitions {
		if p == nil || p.Name != PartitionLabel {
			continue
		}

		return Partition{
			Path:   diskPath,
			Offset: int64(p.Start) * sectorSize,
			Size:   int64(p.End-p.Start+1) * sectorSize,
		}, nil
	}

	return Partition{}, fmt.Errorf("no partition labelled %s found on %s", PartitionLabel, diskPath)
}

// ReadTag returns the value stored under tag, if any.
func ReadTag(diskPath string, tag uint8) ([]byte, bool, error) {
	p, err := Locate(diskPath)
	if err != nil {
		return nil, false, err
	}

	f, err := os.Open(diskPath)
	if err != nil {
		return nil, false, fmt.Errorf("opening %s: %w", diskPath, err)
	}
	defer f.Close()

	talosADV, legacyADV, err := load(f, p)
	if err != nil {
		return nil, false, err
	}

	if value, ok := talosADV.ReadTagBytes(tag); ok {
		return value, true, nil
	}

	value, ok := legacyADV.ReadTagBytes(tag)

	return value, ok, nil
}

// WriteTag stores value under tag, keeping every other tag intact, and flushes
// both ADV copies to the partition.
func WriteTag(diskPath string, tag uint8, value []byte) error {
	p, err := Locate(diskPath)
	if err != nil {
		return err
	}

	f, err := os.OpenFile(diskPath, os.O_RDWR, 0)
	if err != nil {
		return fmt.Errorf("opening %s for writing: %w", diskPath, err)
	}
	defer f.Close()

	talosADV, legacyADV, err := load(f, p)
	if err != nil {
		return err
	}

	if !talosADV.SetTagBytes(tag, value) {
		return fmt.Errorf("value of %d bytes does not fit in the META partition", len(value))
	}

	return flush(f, p, talosADV, legacyADV)
}

func load(f *os.File, p Partition) (adv.ADV, adv.ADV, error) {
	if p.Size < advtalos.Size+2*syslinux.AdvSize {
		return nil, nil, fmt.Errorf("META partition is %d bytes, smaller than the %d bytes Talos needs", p.Size, advtalos.Size+2*syslinux.AdvSize)
	}

	section := io.NewSectionReader(f, p.Offset, p.Size)

	talosADV, err := advtalos.NewADV(section)
	if talosADV == nil && err != nil {
		return nil, nil, fmt.Errorf("loading Talos ADV: %w", err)
	}

	// A non-nil ADV with an error means the partition holds no valid ADV yet
	// (fresh image); Talos starts from an empty one in that case, and so do we.

	legacyADV, err := syslinux.NewADV(section)
	if err != nil {
		return nil, nil, fmt.Errorf("loading syslinux ADV: %w", err)
	}

	return talosADV, legacyADV, nil
}

func flush(f *os.File, p Partition, talosADV, legacyADV adv.ADV) error {
	serialized, err := talosADV.Bytes()
	if err != nil {
		return fmt.Errorf("serializing Talos ADV: %w", err)
	}

	if err := writeAt(f, p.Offset, serialized); err != nil {
		return err
	}

	legacy, err := legacyADV.Bytes()
	if err != nil {
		return fmt.Errorf("serializing syslinux ADV: %w", err)
	}

	if err := writeAt(f, p.Offset+p.Size-int64(len(legacy)), legacy); err != nil {
		return err
	}

	if err := f.Sync(); err != nil {
		return fmt.Errorf("syncing %s: %w", f.Name(), err)
	}

	return nil
}

func writeAt(f *os.File, offset int64, data []byte) error {
	n, err := f.WriteAt(data, offset)
	if err != nil {
		return fmt.Errorf("writing META at offset %d: %w", offset, err)
	}

	if n != len(data) {
		return errors.New("short write to META partition")
	}

	return nil
}
