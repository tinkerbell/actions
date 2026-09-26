package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/url"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/cenkalti/backoff"
	"github.com/dustin/go-humanize"
	"github.com/lmittmann/tint"
	"github.com/mattn/go-isatty"
	"github.com/tinkerbell/actions/image2disk/image"
)

const (
	defaultRetryDuration    = 10
	defaultProgressInterval = 3
)

func main() {
	ctx, done := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGHUP, syscall.SIGTERM)
	defer done()

	disk := os.Getenv("DEST_DISK")
	img := os.Getenv("IMG_URL")
	compressedEnv := os.Getenv("COMPRESSED")
	retryEnabled := os.Getenv("RETRY_ENABLED")
	retryDuration := os.Getenv("RETRY_DURATION_MINUTES")
	progressInterval := os.Getenv("PROGRESS_INTERVAL_SECONDS")
	textLogging := os.Getenv("TEXT_LOGGING")

	log := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{AddSource: true}))
	if tlog, _ := strconv.ParseBool(textLogging); tlog {
		w := os.Stderr
		log = slog.New(tint.NewHandler(w, &tint.Options{
			NoColor: !isatty.IsTerminal(w.Fd()),
		}))
	}

	log.Info("IMAGE2DISK - Cloud image streamer")

	if img == "" {
		log.Error("IMG_URL is required", "image", img)
		os.Exit(1) //nolint:gocritic // deferred signal context cancellation is unnecessary on exit
	}

	if disk == "" {
		log.Error("DEST_DISK is required", "disk", disk)
		os.Exit(1)
	}

	u, err := url.Parse(img)
	if err != nil {
		log.Error("error parsing image URL (IMG_URL)", "err", err, "image", img)
		os.Exit(1)
	}
	// We can ignore the error and default compressed to false.
	cmp, _ := strconv.ParseBool(compressedEnv)
	re, _ := strconv.ParseBool(retryEnabled)
	pi, err := strconv.Atoi(progressInterval)
	if err != nil || pi < 0 {
		pi = defaultProgressInterval
	}

	opts := image.Options{
		Logger: log,
		// A zero interval deliberately disables progress logging rather than
		// panicking in time.NewTicker, which is what it used to do.
		ProgressInterval: time.Duration(pi) * time.Second,
		Compressed:       cmp,
		Mode:             image.ModePlain,
		MemoryBudget:     envInt64("MEMORY_BUDGET_BYTES"),
		ChunkSize:        int(envInt64("WRITE_CHUNK_SIZE_BYTES")),
		Writers:          int(envInt64("WRITE_WORKERS")),
		QueueDepth:       int(envInt64("QUEUE_DEPTH")),
		DiffBlockSize:    int(envInt64("DIFF_BLOCK_SIZE_BYTES")),
	}
	if changed, _ := strconv.ParseBool(os.Getenv("WRITE_CHANGED_BLOCKS_ONLY")); changed {
		opts.Mode = image.ModeDiff
	}

	var res image.Result
	operation := func() error {
		var opErr error
		res, opErr = image.Write(ctx, u.String(), disk, opts)
		if opErr != nil {
			return fmt.Errorf("error writing image to disk: %w", opErr)
		}
		return nil
	}

	if re {
		log.Info("retrying of image2disk is enabled")
		boff := backoff.NewExponentialBackOff()
		rd, err := strconv.Atoi(retryDuration)
		if err != nil {
			rd = defaultRetryDuration
			if retryDuration == "" {
				log.Info(fmt.Sprintf("no retry duration specified, using %v minutes for retry duration", rd))
			} else {
				log.Info(fmt.Sprintf("error converting retry duration to integer, using %v minutes for retry duration", rd), "err", err)
			}
		}
		boff.MaxElapsedTime = time.Duration(rd) * time.Minute
		bctx := backoff.WithContext(boff, ctx)
		retryNotifier := func(err error, duration time.Duration) {
			log.Error("retrying image2disk", "err", err, "duration", duration)
		}
		// try to write the image to disk with exponential backoff for 10 minutes
		if err := backoff.RetryNotify(operation, bctx, retryNotifier); err != nil {
			log.Error("error writing image to disk", "err", err, "image", img, "disk", disk)
			os.Exit(1)
		}
	} else {
		// try to write the image to disk without retry
		if err := operation(); err != nil {
			log.Error("error writing image to disk", "err", err, "image", img, "disk", disk)
			os.Exit(1)
		}
	}

	attrs := []any{
		"image", img,
		"disk", disk,
		"format", res.Format,
		"mode", string(res.Mode),
		"decoded", humanize.IBytes(uint64(res.BytesDecoded)),
		"written", humanize.IBytes(uint64(res.BytesWritten)),
		"writeCalls", res.WriteCalls,
		"duration", res.Duration.Round(time.Millisecond).String(),
	}
	if res.Mode == image.ModeDiff {
		attrs = append(attrs, "skipped", humanize.IBytes(uint64(res.BytesSkipped)))
		if touched := res.BytesWritten + res.BytesSkipped; touched > 0 {
			attrs = append(attrs, "skippedPct", fmt.Sprintf("%.1f%%", float64(res.BytesSkipped)/float64(touched)*100))
		}
		if res.DiffFallbacks > 0 {
			attrs = append(attrs, "readBackFailures", res.DiffFallbacks)
		}
	}
	log.Info("Successfully wrote image to disk", attrs...)
}

// envInt64 reads a non-negative integer environment variable, returning zero
// when it is unset or unparseable so that the caller's default applies.
func envInt64(name string) int64 {
	v, err := strconv.ParseInt(os.Getenv(name), 10, 64)
	if err != nil || v < 0 {
		return 0
	}
	return v
}
