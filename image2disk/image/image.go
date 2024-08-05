package image

// This package handles the pulling and management of images

import (
	"compress/bzip2"
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/dustin/go-humanize"
	"github.com/klauspost/compress/zstd"
	log "github.com/sirupsen/logrus"
	"github.com/ulikunitz/xz"
	"golang.org/x/sys/unix"
)

// WriteCounter counts the number of bytes written to it. It implements to the io.Writer interface
// and we can pass this into io.TeeReader() which will report progress on each write cycle.
type WriteCounter struct {
	Total uint64
}

func (wc *WriteCounter) Write(p []byte) (int, error) {
	n := len(p)
	wc.Total += uint64(n)
	return n, nil
}

func tickerProgress(byteCounter uint64) {
	// Clear the line by using a character return to go back to the start and remove
	// the remaining characters by filling it with spaces
	fmt.Printf("\r%s", strings.Repeat(" ", 35))

	// Return again and print current status of download
	// We use the humanize package to print the bytes in a meaningful way (e.g. 10 MB)
	fmt.Printf("\rDownloading... %s complete", humanize.Bytes(byteCounter))
}

// Write will pull an image and write it to local storage device
// with compress set to true it will use gzip compression to expand the data before
// writing to an underlying device.
func Write(sourceImage, destinationDevice string, compressed bool) error {
	const maxRetries = 5
	const retryDelay = 10 * time.Second

	var err error
	var resp *http.Response
	var fileOut *os.File
	var out io.Reader

	for attempt := 0; attempt < maxRetries; attempt++ {
		resp, err = fetchImage(sourceImage)
		if err != nil {
			if attempt < maxRetries-1 {
				log.Warnf("Failed to fetch image [%s]: %v. Retrying in %v...", sourceImage, err, retryDelay)
				time.Sleep(retryDelay)
				continue
			}
			return fmt.Errorf("failed to fetch image [%s] after %d retries: %w", sourceImage, maxRetries, err)
		}
		defer resp.Body.Close() // Ensure body is closed when done

		fileOut, err = os.OpenFile(destinationDevice, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o644)
		if err != nil {
			return err
		}
		defer fileOut.Close()

		if !compressed {
			// Without compression send raw output
			out = resp.Body
		} else {
			// Find compression algorithm based upon extension
			decompressor, err := findDecompressor(sourceImage, resp.Body)
			if err != nil {
				return err
			}
			defer decompressor.Close()
			out = decompressor
		}

		log.Infof("Beginning write of image [%s] to disk [%s]", filepath.Base(sourceImage), destinationDevice)

		ticker := time.NewTicker(500 * time.Millisecond)
		counter := &WriteCounter{}
		go func() {
			for range ticker.C {
				tickerProgress(counter.Total)
			}
		}()

		if _, err = io.Copy(fileOut, io.TeeReader(out, counter)); err != nil {
			ticker.Stop()
			if attempt < maxRetries-1 {
				log.Warnf("Failed to write data, retrying... [%v]", err)
				time.Sleep(retryDelay)
				continue
			}
			return fmt.Errorf("failed to write image after %d retries: %w", maxRetries, err)
		}

		ticker.Stop()
		fmt.Printf("\n")

		// Do the equivalent of partprobe on the device
		if err := fileOut.Sync(); err != nil {
			return fmt.Errorf("failed to sync the block device: %w", err)
		}

		if err := unix.IoctlSetInt(int(fileOut.Fd()), unix.BLKRRPART, 0); err != nil {
			// Ignore errors since it may be a partition, but log in case it's helpful
			log.Errorf("Error re-probing the partitions for the specified device: %v", err)
		}

		return nil
	}

	return fmt.Errorf("failed to write image after %d retries", maxRetries)
}

// fetchImage fetches the image from the specified URL.
func fetchImage(sourceImage string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(context.TODO(), "GET", sourceImage, nil)
	if err != nil {
		return nil, err
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode > 300 {
		// Customize response for the 404 to make debugging simpler
		if resp.StatusCode == 404 {
			return nil, fmt.Errorf("%s not found", sourceImage)
		}
		return nil, fmt.Errorf("%s", resp.Status)
	}

	return resp, nil
}

func findDecompressor(imageURL string, r io.Reader) (io.ReadCloser, error) {
	switch filepath.Ext(imageURL) {
	case ".bzip2", ".bz2":
		return io.NopCloser(bzip2.NewReader(r)), nil
	case ".gz":
		reader, err := gzip.NewReader(r)
		if err != nil {
			return nil, fmt.Errorf("[ERROR] New gzip reader: %w", err)
		}
		return reader, nil
	case ".xz":
		reader, err := xz.NewReader(r)
		if err != nil {
			return nil, fmt.Errorf("[ERROR] New xz reader: %w", err)
		}
		return io.NopCloser(reader), nil
	case ".zs":
		reader, err := zstd.NewReader(r)
		if err != nil {
			return nil, fmt.Errorf("[ERROR] New zs reader: %w", err)
		}
		return reader.IOReadCloser(), nil
	}

	return nil, fmt.Errorf("unknown compression suffix [%s]", filepath.Ext(imageURL))
}
