package main

import (
	"fmt"
	"log/slog"
	"os"
	"strconv"
	"time"

	"github.com/cenkalti/backoff"
	"github.com/tinkerbell/actions/image2disk/image"
)

func main() {
	log := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{AddSource: true}))
	log.Info("IMAGE2DISK - Cloud image streamer")
	disk := os.Getenv("DEST_DISK")
	img := os.Getenv("IMG_URL")
	compressedEnv := os.Getenv("COMPRESSED")
	retryDuration := os.Getenv("RETRY_DURATION_MINUTES")

	// We can ignore the error and default compressed to false.
	cmp, _ := strconv.ParseBool(compressedEnv)

	operation := func() error {
		if err := image.Write(img, disk, cmp); err != nil {
			return fmt.Errorf("error writing image to disk: %w", err)
		}
		return nil
	}
	boff := backoff.NewExponentialBackOff()
	rd, err := strconv.Atoi(retryDuration)
	if err != nil {
		log.Error("error converting retry duration to integer, using 10 minutes for retry duration", "err", err)
		rd = 10
	}
	boff.MaxElapsedTime = time.Duration(rd) * time.Minute
	// try to write the image to disk with exponential backoff for 10 minutes
	if err := backoff.Retry(operation, boff); err != nil {
		log.Error("error writing image to disk", "err", err)
		os.Exit(1)
	}
	log.Info("Successfully wrote image to disk", "image", img, "disk", disk)
}
