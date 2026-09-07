package main

import (
	"fmt"
	"os"
	"strconv"

	log "github.com/sirupsen/logrus"
	"github.com/tinkerbell/actions/oci2disk/image"
)

func main() {
	fmt.Printf("OCI2DISK - OCI Container Disk image streamer\n------------------------\n")
	disk := os.Getenv("DEST_DISK")
	img := os.Getenv("IMG_URL")
	registryUsername := os.Getenv("REGISTRY_USERNAME")
	registryPassword := os.Getenv("REGISTRY_PASSWORD")
	// We can ignore the error and default to false.
	skipVerify, _ := strconv.ParseBool(os.Getenv("SKIP_VERIFY"))

	// Write the image to disk (platform and compression auto-detected)
	err := image.Write(img, disk, registryUsername, registryPassword, skipVerify)
	if err != nil {
		log.Fatal(err)
	}
}
