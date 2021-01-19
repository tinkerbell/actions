package main

import (
	"fmt"
	"os"

	log "github.com/sirupsen/logrus"

	"github.com/tinkerbell/hub/actions/stream/v1/pkg/image"
)

func main() {

	fmt.Printf("STREAM - Cloud image streamer\n------------------------\n")
	disk := os.Getenv("DEST_DISK")
	img := os.Getenv("IMG_URL")
	compressedEnv := os.Getenv("COMPRESSED")
	var cmp bool
	if compressedEnv != "" {
		cmp = true
	}
	err := image.Write(img, disk, cmp)
	if err != nil {
		log.Fatal(err)
	}
	log.Info("Successfully written [%s] to [%s]", img, disk)
}
