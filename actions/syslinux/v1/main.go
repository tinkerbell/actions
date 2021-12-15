package main

import (
	"fmt"
	"io"
	"os"
	"os/exec"

	log "github.com/sirupsen/logrus"
	"golang.org/x/sys/unix"
)

func main() {
	fmt.Printf("SYSLINUX - Boot Loader Installation\n------------------------\n")
	disk := os.Getenv("DEST_DISK")
	partition := os.Getenv("DEST_PARTITION")

	ver := os.Getenv("SYSLINUX_VERSION")

	switch ver {
	case "386", "3.86":
		syslinux386(disk, partition)
	default:
		log.Fatalf("Unknown syslinux version [%s]", ver)
	}
}

func syslinux386(disk, partition string) {
	log.Infof("Writing mbr to [%s] and installing boot loader to [%s]", disk, partition)
	// Open the block device and write the Master boot record
	blockOut, err := os.OpenFile(disk, os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		log.Fatalln(err)
	}
	ReReadPartitionTable(blockOut)
	defer blockOut.Close()

	mbrIn, err := os.OpenFile("/mbr.bin.386", os.O_RDONLY, 0o644)
	defer mbrIn.Close()

	_, err = io.Copy(blockOut, mbrIn)
	if err != nil {
		log.Fatalln(err)
	}

	_, err = os.Stat(partition)
	if err != nil {
		log.Fatalln(err)
	}

	cmd := exec.Command("/syslinux.386", partition)
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
	err = cmd.Start()
	if err != nil {
		log.Fatalf("Error starting [%v]", err)
	}
	err = cmd.Wait()
	if err != nil {
		log.Fatalf("Error running [%v]", err)
	}
}

const (
	BLKRRPART = 0x125f
)

// ReReadPartitionTable forces the kernel to re-read the partition table
// on the disk.
//
// It is done via an ioctl call with request as BLKRRPART.
func ReReadPartitionTable(d *os.File) error {
	fd := d.Fd()
	_, err := unix.IoctlGetInt(int(fd), BLKRRPART)
	if err != nil {
		return fmt.Errorf("Unable to re-read partition table: %v", err)
	}
	return nil
}
