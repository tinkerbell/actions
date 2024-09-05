package main

import (
	"fmt"
	"os"

	log "github.com/sirupsen/logrus"
	"github.com/tinkerbell/actions/grub2disk/grub"
)

func main() {
	fmt.Printf("GRUB2Disk - GRUB streamer\n------------------------\n")
	grubInstallPath := os.Getenv("GRUB_INSTALL_PATH")
	grubBlockDevice := os.Getenv("GRUB_DISK")
	filesystemType := os.Getenv("FS_TYPE")

	if err := grub.MountGrub(grubInstallPath, grubBlockDevice, filesystemType); err != nil {
		log.Fatal(err)
	}
	log.Infof("grub successfully written on [%s]", grubInstallPath)
}
