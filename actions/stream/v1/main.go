package main

import (
	"fmt"
	"os"
	"strconv"

	log "github.com/sirupsen/logrus"

	"github.com/tinkerbell/hub/actions/stream/v1/pkg/image"
)

func main() {

	fmt.Printf("STREAM - Cloud image streamer\n------------------------\n")
	disk := os.Getenv("DEST_DISK")
	img := os.Getenv("IMG_URL")
	compressedEnv := os.Getenv("COMPRESSED")

	// We can ignore the error and default compressed to false.
	cmp, _ := strconv.ParseBool(compressedEnv)

	// Write the image to disk
	err := image.Write(img, disk, cmp)
	if err != nil {
		log.Fatal(err)
	}
	log.Info("Successfully written [%s] to [%s]", img, disk)
}
