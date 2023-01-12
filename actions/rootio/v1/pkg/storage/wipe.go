package storage

import (
	"os"

	log "github.com/sirupsen/logrus"
	"github.com/tinkerbell/hub/actions/rootio/v1/pkg/types.go"
)

// Wipe will clean the table from a disk.
func Wipe(d types.Disk) error {
	disk, err := os.OpenFile(d.Device, os.O_CREATE|os.O_WRONLY, 0o644)
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
