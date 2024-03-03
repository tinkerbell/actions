package app

import (
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"syscall"

	"github.com/go-logr/zerologr"
	"github.com/rs/zerolog"
	"github.com/tinkerbell/actions/archive2disk/checksum"
	"github.com/tinkerbell/actions/archive2disk/download"
	"github.com/tinkerbell/actions/archive2disk/tar"
)

const mountPoint = "/archive2disk"

// Archive2Disk is the entrypoint for the Archive2Disk action.
func Archive2Disk(out io.Writer) error {
	zl := zerolog.New(out)
	l := zerologr.New(&zl)

	l.Info("Archive2Disk")

	cfg, err := ReadConfig()
	if err != nil {
		return err
	}

	if err := os.Mkdir(mountPoint, os.ModeDir); err != nil {
		return fmt.Errorf("create mount directory (%v): %v", mountPoint, err)
	}

	// Mount the block device to the /mountPoint point
	err = syscall.Mount(cfg.DevicePath, mountPoint, cfg.DeviceFilesystem, 0, "")
	if err != nil {
		return fmt.Errorf("mount device path (%v): %v", cfg.DevicePath, err)
	}
	l.Info("Mounted %s at %s", cfg.DevicePath, mountPoint)

	destination, err := filepath.Abs(filepath.Join(mountPoint, cfg.DestinationPath))
	if err != nil {
		return err
	}

	source, err := download.From(cfg.ArchiveURL, download.WithTimeout(cfg.HTTPClientTimeout))
	if err != nil {
		return fmt.Errorf("download archive: %v", err)
	}
	defer source.Close()

	reader, verifyDigest, err := checksum.VerifyStream(source, cfg.ArchiveChecksum)
	if err != nil {
		return fmt.Errorf("create checksum verifier: %v", err)
	}

	if cfg.ArchiveType == "targz" {
		gr, err := gzip.NewReader(reader)
		if err != nil {
			return fmt.Errorf("create decompressor: %v", err)
		}
		defer gr.Close()
		reader = gr
	}

	if err := tar.Extract(reader, destination); err != nil {
		return err
	}

	if !verifyDigest() {
		return fmt.Errorf("digest mismatch: expected %v", cfg.ArchiveChecksum)
	}

	l.Info("Successfully unpacked [%s] to [%s] on device [%s]", cfg.ArchiveURL, cfg.DestinationPath, cfg.DevicePath)

	return nil
}
