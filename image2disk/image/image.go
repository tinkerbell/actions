package image

// This package handles the pulling and management of images

import (
	"compress/bzip2"
	"compress/gzip"
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"math"
	"net/http"
	"os"
	"path/filepath"
	"sync/atomic"
	"time"

	"github.com/klauspost/compress/zstd"
	"github.com/ulikunitz/xz"
	"golang.org/x/sys/unix"
)

type Progress struct {
	w      io.Writer
	r      io.Reader
	wBytes atomic.Int64
	rBytes atomic.Int64
}

func NewProgress(w io.Writer, r io.Reader) *Progress {
	return &Progress{w: w, r: r}
}

func (p *Progress) Write(b []byte) (n int, err error) {
	nu, err := p.w.Write(b)
	if err != nil {
		p.wBytes.Add(int64(nu))
		return nu, fmt.Errorf("error with write: %w", err)
	}
	p.wBytes.Add(int64(nu))
	return nu, nil
}

func (p *Progress) Read(b []byte) (n int, err error) {
	nu, err := p.r.Read(b)
	if err != nil {
		p.rBytes.Add(int64(nu))
		return nu, fmt.Errorf("error with read: %w", err)
	}
	p.rBytes.Add(int64(nu))
	return nu, nil
}

func (p *Progress) readBytes() int64 {
	return p.rBytes.Load()
}

func (p *Progress) writeBytes() int64 {
	return p.wBytes.Load()
}

func prettyByteSize(b int64) string {
	bf := float64(b)
	for _, unit := range []string{"", "Ki", "Mi", "Gi", "Ti", "Pi", "Ei", "Zi"} {
		if math.Abs(bf) < 1024.0 {
			return fmt.Sprintf("%3.6f%sB", bf, unit)
		}
		bf /= 1024.0
	}
	return fmt.Sprintf("%.6fYiB", bf)
}

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

// Write will pull an image and write it to local storage device
// with compress set to true it will use gzip compression to expand the data before
// writing to an underlying device.
func Write(ctx context.Context, log *slog.Logger, sourceImage, destinationDevice string, compressed bool, progressInterval time.Duration) error {
	req, err := http.NewRequestWithContext(ctx, "GET", sourceImage, nil)
	if err != nil {
		return err
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode > 300 {
		// Customize response for the 404 to make debugging simpler
		if resp.StatusCode == 404 {
			return fmt.Errorf("%s not found", sourceImage)
		}
		return fmt.Errorf("%s", resp.Status)
	}

	var out io.Reader

	fileOut, err := os.OpenFile(destinationDevice, os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer fileOut.Close()

	progressRW := NewProgress(fileOut, resp.Body)

	if !compressed {
		// Without compression send raw output
		out = progressRW
	} else {
		// Find compression algorithm based upon extension
		decompressor, err := findDecompressor(sourceImage, progressRW)
		if err != nil {
			return err
		}
		defer decompressor.Close()
		out = decompressor
	}

	log.Info(fmt.Sprintf("Beginning write of image [%s] to disk [%s]", filepath.Base(sourceImage), destinationDevice))
	ticker := time.NewTicker(progressInterval)
	done := make(chan bool)
	go func() {
		totalSize := resp.ContentLength
		for {
			select {
			case <-done:
				log.Info("read and write progress", "written", prettyByteSize(progressRW.writeBytes()), "compressedSize", prettyByteSize(totalSize), "read", prettyByteSize(progressRW.readBytes()))
				return
			case <-ticker.C:
				log.Info("read and write progress", "written", prettyByteSize(progressRW.writeBytes()), "compressedSize", prettyByteSize(totalSize), "read", prettyByteSize(progressRW.readBytes()))
			}
		}
	}()

	count, err := io.Copy(progressRW, out)
	// EOF and ErrUnexpectedEOF can be ignored.
	if err != nil && !errors.Is(err, io.EOF) && !errors.Is(err, io.ErrUnexpectedEOF) {
		ticker.Stop()
		done <- true
		return fmt.Errorf("error writing %s bytes to disk [%s] -> %w", prettyByteSize(count), destinationDevice, err)
	}

	ticker.Stop()
	done <- true

	// Do the equivalent of partprobe on the device
	if err := fileOut.Sync(); err != nil {
		return fmt.Errorf("failed to sync the block device")
	}

	if err := unix.IoctlSetInt(int(fileOut.Fd()), unix.BLKRRPART, 0); err != nil {
		// Ignore errors since it may be a partition, but log in case it's helpful
		log.Info("error re-probing the partitions for the specified device", "err", err)
	}

	return nil
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
	case ".zs", ".zst":
		reader, err := zstd.NewReader(r)
		if err != nil {
			return nil, fmt.Errorf("[ERROR] New zs reader: %w", err)
		}
		return reader.IOReadCloser(), nil
	}

	return nil, fmt.Errorf("unknown compression suffix [%s]", filepath.Ext(imageURL))
}
