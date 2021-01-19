package storage

import (
	"fmt"
	"os"
	"syscall"
	"time"

	log "github.com/sirupsen/logrus"
	"github.com/tinkerbell/hub/actions/rootio/v1/pkg/types.go"

	diskfs "github.com/diskfs/go-diskfs"
	"github.com/diskfs/go-diskfs/partition/gpt"
)

const sectorSize = 512

// VerifyBlockDevice will check that the device actually exists and is a block device
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
	mode := (*d).Sys().(*syscall.Stat_t).Mode
	return (mode & syscall.S_IFMT) == syscall.S_IFBLK
}

// ExamineDisk will look at the configuration of a disk
func ExamineDisk(d types.Disk) error {
	disk, err := diskfs.Open(d.Device)
	if err != nil {
		return err
	}
	log.Infof("Examining disk [%s]", d.Device)
	log.Infof("Disk Size [%dMB]", disk.Size/1024/1024)

	p, err := disk.GetPartitionTable()
	if err != nil {
		return err
	}
	partitions := p.GetPartitions()
	log.Infof("Found [%d] partitions", len(partitions))
	err = disk.File.Sync()
	if err != nil {
		return err
	}
	time.Sleep(time.Second * 2)
	err = disk.File.Close()
	if err != nil {
		return err
	}
	return nil
}

// Partition will create the partitions and write them to the disk
func Partition(d types.Disk) error {

	table := &gpt.Table{
		ProtectiveMBR:      true,
		LogicalSectorSize:  sectorSize,
		PhysicalSectorSize: sectorSize,
	}
	disk, err := diskfs.Open(d.Device)
	if err != nil {
		return err
	}

	// Build the table
	partitionNumber := 1
	var sectorStart uint64
	sectorStart = 2048
	for x := range d.Partitions {
		// Calculate the end sector by adding the starting sec
		sectorEnd := sectorStart + uint64(d.Partitions[x].Size)

		if d.Partitions[x].Number == partitionNumber {
			partitionNumber++
			newPartition := &gpt.Partition{
				Name:  d.Partitions[x].Label,
				Start: sectorStart,
				End:   sectorEnd,
			}
			sectorStart = sectorStart + sectorEnd

			switch d.Partitions[x].Label {
			case "SWAP":
				newPartition.Type = gpt.LinuxSwap
			case "BIOS":
				newPartition.Type = gpt.BiosBoot
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
	err = disk.File.Sync()
	if err != nil {
		return err
	}
	err = disk.File.Close()
	if err != nil {
		return err
	}
	return nil
}

// Wipe will clean the table from a disk
func Wipe(d types.Disk) error {
	disk, err := os.OpenFile(d.Device, os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer disk.Close()
	bigBuff := make([]byte, 1024*1024*1024)
	n, err := disk.Write(bigBuff)
	if err != nil {
		return err
	}
	log.Infof("Wrote [%d] bytes to [%s]", n, d.Device)
	log.Infoln("Flushing writes to new partition")
	err = disk.Sync()
	if err != nil {
		return err
	}
	err = disk.Close()
	if err != nil {
		return err
	}
	return nil
}
