package main

import (
	"fmt"
	"os"

	log "github.com/sirupsen/logrus"
	"github.com/tinkerbell/actions/oci2disk/image"
)

func main() {
	fmt.Printf("OCI2DISK - OCI Container Disk image streamer\n------------------------\n")
	disk := os.Getenv("DEST_DISK")
	img := os.Getenv("IMG_URL")

	// Write the image to disk (platform auto-detected)
	err := image.Write(img, disk)
	if err != nil {
		log.Fatal(err)
	}
	log.Infof("Successfully written [%s] to [%s]", img, disk)
}
