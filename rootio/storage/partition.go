package storage

import (
	"fmt"
	"os"
	"syscall"
	"time"

	diskfs "github.com/diskfs/go-diskfs"
	"github.com/diskfs/go-diskfs/partition/gpt"
	"github.com/diskfs/go-diskfs/partition/mbr"
	log "github.com/sirupsen/logrus"
)

const sectorSize = 512

// VerifyBlockDevice will check that the device actually exists and is a block device.
func VerifyBlockDevice(device string) error {
	d, err := os.Stat(device)
	if os.IsNotExist(err) {
		return fmt.Errorf("%s does not exist", device)
	}
	if !isBlockDevice(&d) {
		return fmt.Errorf("%s is not a block device", device)
	}
	// passed checks
	return nil
}

func isBlockDevice(d *os.FileInfo) bool {
	// this probably shouldn't be so hard
	// but d.Mode()&os.ModeDevice == 0 doesn't work as expected
	stat, ok := (*d).Sys().(*syscall.Stat_t)
	if !ok {
		return false
	}
	return (stat.Mode & syscall.S_IFMT) == syscall.S_IFBLK
}

// ExamineDisk will look at the configuration of a disk.
func ExamineDisk(d Disk) (err error) {
	disk, err := diskfs.Open(d.Device)
	if err != nil {
		return err
	}
	defer func() {
		if cerr := disk.Close(); cerr != nil && err == nil {
			err = cerr
		}
	}()
	log.Infof("Examining disk [%s]", d.Device)
	log.Infof("Disk Size [%dMB]", disk.Size/1024/1024)

	p, err := disk.GetPartitionTable()
	if err != nil {
		return err
	}
	partitions := p.GetPartitions()
	log.Infof("Found [%d] partitions", len(partitions))
	f, err := disk.Backend.Sys()
	if err != nil {
		return err
	}
	if err := f.Sync(); err != nil {
		return err
	}
	time.Sleep(time.Second * 2)
	return nil
}

// Partition will create the partitions and write them to the disk.
func Partition(d Disk) (err error) {
	table := &gpt.Table{
		ProtectiveMBR:      true,
		LogicalSectorSize:  sectorSize,
		PhysicalSectorSize: sectorSize,
	}
	disk, err := diskfs.Open(d.Device)
	if err != nil {
		return err
	}
	defer func() {
		if cerr := disk.Close(); cerr != nil && err == nil {
			err = cerr
		}
	}()

	// Build the table
	partitionNumber := 1
	var sectorStart uint64
	sectorStart = 2048
	for x := range d.Partitions {
		// Calculate the end sector by adding the starting sec
		sectorEnd := sectorStart + d.Partitions[x].Size

		if d.Partitions[x].Number == partitionNumber {
			partitionNumber++
			newPartition := &gpt.Partition{
				Name:  d.Partitions[x].Label,
				Start: sectorStart,
				End:   sectorEnd,
			}

			sectorStart = sectorEnd + 1

			switch d.Partitions[x].Label {
			case "SWAP":
				newPartition.Type = gpt.LinuxSwap
			case "BIOS":
				newPartition.Type = gpt.BIOSBoot
			case "EFI":
				newPartition.Type = gpt.EFISystemPartition
			default:
				newPartition.Type = gpt.LinuxFilesystem
			}

			// If this is set to 0 then use the remaining disk
			if d.Partitions[x].Size == 0 {
				newPartition.End = uint64(disk.Size/sectorSize) - 2048
			}

			log.Infof("New Partition Name=%s Start=%d End=%d", newPartition.Name, newPartition.Start, newPartition.End)
			table.Partitions = append(table.Partitions, newPartition)
		}
	}

	err = disk.Partition(table)
	if err != nil {
		return err
	}
	log.Infoln("Flushing writes to new partition")
	f, err := disk.Backend.Sys()
	if err != nil {
		return err
	}
	return f.Sync()
}

// MBRPartition will create the partitions and write them to the disk.
func MBRPartition(d Disk) (err error) {
	table := &mbr.Table{
		LogicalSectorSize:  sectorSize,
		PhysicalSectorSize: sectorSize,
	}
	disk, err := diskfs.Open(d.Device)
	if err != nil {
		return err
	}
	defer func() {
		if cerr := disk.Close(); cerr != nil && err == nil {
			err = cerr
		}
	}()

	// Build the table
	partitionNumber := 1
	var sectorStart uint32
	sectorStart = 2048
	for x := range d.Partitions {
		// sector start is for bootloader
		remainingSectors := disk.Size/sectorSize - int64(sectorStart)

		if d.Partitions[x].Number == partitionNumber {
			partitionNumber++
			newPartition := &mbr.Partition{
				Start: sectorStart,
				Size:  uint32(d.Partitions[x].Size),
			}

			sectorStart += uint32(d.Partitions[x].Size)

			switch d.Partitions[x].Label {
			case "SWAP":
				newPartition.Type = 0x82
			case "LINUX":
				newPartition.Type = mbr.Linux
			case "LINUX_ACTIVE":
				newPartition.Type = mbr.Linux
				newPartition.Bootable = true
			case "FAT32":
				newPartition.Type = mbr.Fat32LBA
			case "FAT32_ACTIVE":
				newPartition.Type = mbr.Fat32LBA
				newPartition.Bootable = true
			default:
				newPartition.Type = mbr.Linux
			}

			// If this is set to 0 then use the remaining disk
			if d.Partitions[x].Size == 0 {
				newPartition.Size = uint32(remainingSectors)
			}

			log.Infof("New Partition Name=%s Start=%d Size=%d", d.Partitions[x].Label, newPartition.Start, newPartition.Size)
			table.Partitions = append(table.Partitions, newPartition)
		}
	}

	err = disk.Partition(table)
	if err != nil {
		return err
	}
	log.Infoln("Flushing writes to new partition")
	f, err := disk.Backend.Sys()
	if err != nil {
		return err
	}
	return f.Sync()
}
