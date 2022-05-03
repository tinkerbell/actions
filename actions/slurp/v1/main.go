package main

import (
	"fmt"
	"os"
	"strconv"

	log "github.com/sirupsen/logrus"
	"github.com/tinkerbell/hub/actions/slurp/v1/pkg/image"
)

func main() {
	fmt.Printf("SLURP - Upload the contents of a block device\n------------------------\n")
	sourceDisk := os.Getenv("SOURCE_DISK")
	destinationURL := os.Getenv("DEST_URL")
	imageName := os.Getenv("IMG_NAME")
	compressed := os.Getenv("COMPRESSED")

	// We can ignore the error and default compressed to false.
	cmp, _ := strconv.ParseBool(compressed)

	// Write the image to disk
	err := image.Read(sourceDisk, destinationURL, imageName, cmp)
	if err != nil {
		log.Fatal(err)
	}
	log.Infof("Successfully uploaded contents of  [%s] to [%s]", sourceDisk, destinationURL)
}
