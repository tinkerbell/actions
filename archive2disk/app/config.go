package app

import (
	"fmt"
	"os"
	"time"
)

const (
	DevicePathEnv        = "DEST_DISK"
	DeviceFilesystemEnv  = "FS_TYPE"
	destinationPathEnv   = "DEST_PATH"
	archiveURLEnv        = "ARCHIVE_URL"
	archiveTypeEnv       = "ARCHIVE_TYPE"
	archiveChecksum      = "TARFILE_CHECKSUM"
	httpClientTimeoutEnv = "HTTP_CLIENT_TIMEOUT_MINUTES"
)

type Config struct {
	// DevicePath is the path to the block device Device that the archive will be mounted
	// and written to. E.g. /dev/sda1
	DevicePath string

	// DeviceFilesystem is the filesystem on DevicePath.
	DeviceFilesystem string

	// DestinationPath is the path within the filesystem of DevicePath that the archive will
	// be extracted to.
	DestinationPath string

	// ArchiveURL is the URL to the archive.
	ArchiveURL string

	// ArchiveType is the kind of archive. Options are tar, targz. Defauts to tar.
	ArchiveType string

	// ArchiveChecksum is the checksum of the archive. When unset, no checksum is validated.
	// Format: <algorithm>:<hash>. For example, sha256:123...
	ArchiveChecksum string

	// HTTPClientTimeout is the timeout for the http client that downloads the archive.
	HTTPClientTimeout time.Duration
}

func ReadConfig() (Config, error) {
	cfg := Config{
		DevicePath:       os.Getenv(DevicePathEnv),
		DeviceFilesystem: os.Getenv(DeviceFilesystemEnv),
		DestinationPath:  os.Getenv(destinationPathEnv),
		ArchiveURL:       os.Getenv(archiveURLEnv),
		ArchiveType:      os.Getenv(archiveTypeEnv),
		ArchiveChecksum:  os.Getenv(archiveChecksum),
	}

	timeout := 5 * time.Minute
	if v, exists := os.LookupEnv(httpClientTimeoutEnv); exists {
		t, err := time.ParseDuration(fmt.Sprintf("%vm", v))
		if err != nil {
			return cfg, fmt.Errorf("read cfg: %v: not an integer", httpClientTimeoutEnv)
		}
		timeout = t
	}
	if timeout < 0 {
		return cfg, fmt.Errorf("read cfg: %v: cannot be negative", httpClientTimeoutEnv)
	}
	cfg.HTTPClientTimeout = timeout

	return cfg, nil
}
