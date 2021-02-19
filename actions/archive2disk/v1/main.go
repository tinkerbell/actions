package main

import (
	"fmt"
	"os"
	"path/filepath"
	"syscall"

	log "github.com/sirupsen/logrus"
	"github.com/tinkerbell/hub/actions/archive2disk/v1/pkg/archive"
)

const mountAction = "/mountAction"

func main() {

	fmt.Printf("Archive2Disk - Cloud archive streamer\n------------------------\n")
	blockDevice := os.Getenv("BLOCK_DEVICE")
	filesystemType := os.Getenv("FS_TYPE")
	path := os.Getenv("DEST_PATH")

	archiveURL := os.Getenv("ARCHIVE_URL")
	archiveType := os.Getenv("ARCHIVE_TYPE")

	if blockDevice == "" {
		log.Fatalf("No Block Device speified with Environment Variable [BLOCK_DEVICE]")
	}

	// Create the /mountAction mountpoint (no folders exist previously in scratch container)
	err := os.Mkdir(mountAction, os.ModeDir)
	if err != nil {
		log.Fatalf("Error creating the action Mountpoint [%s]", mountAction)
	}

	// Mount the block device to the /mountAction point
	err = syscall.Mount(blockDevice, mountAction, filesystemType, 0, "")
	if err != nil {
		log.Fatalf("Mounting [%s] -> [%s] error [%v]", blockDevice, mountAction, err)
	}
	log.Infof("Mounted [%s] -> [%s]", blockDevice, mountAction)

	// Write the image to disk
<<<<<<< HEAD
	err = archive.Write(archiveURL, archiveType, filepath.Join(mountAction, "path"))
	if err != nil {
		log.Fatal(err)
	}
	log.Info("Successfully unpacked [%s] to [%s] on device [%s]", archiveURL, path, blockDevice)
=======
	err = archive.Write(archiveURL, archiveType, filepath.Join(mountAction, path))
	if err != nil {
		log.Fatal(err)
	}
	log.Infof("Successfully unpacked [%s] to [%s] on device [%s]", archiveURL, path, blockDevice)
>>>>>>> upstream/main
}
