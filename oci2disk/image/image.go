package image

// This package handles the pulling and management of images

import (
	"compress/bzip2"
	"compress/gzip"
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"

	"github.com/containerd/containerd/reference"
	"github.com/containerd/containerd/remotes/docker"
	"github.com/deislabs/oras/pkg/oras"
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

type Compression int

const (
	CompressionNone Compression = iota
	CompressionGzip
	CompressionXZ
	CompressionZstd
	CompressionBzip2
)

// ParseCompression parses a compression string into a Compression type.
// Accepts: gz, gzip, xz, zstd, zst, bzip2
// For backward compatibility: "true" detects compression from the imageURL extension.
// Empty string or "false" returns CompressionNone.
func ParseCompression(s string, imageURL string) Compression {
	switch s {
	case "gz", "gzip":
		return CompressionGzip
	case "xz":
		return CompressionXZ
	case "zstd", "zst", "zs":
		return CompressionZstd
	case "bzip2":
		return CompressionBzip2
	case "true":
		// Backward compatibility: detect from image URL extension
		return detectCompressionFromURL(imageURL)
	default:
		return CompressionNone
	}
}

// detectCompressionFromURL detects compression type from URL file extension.
// This provides backward compatibility with COMPRESSED=true.
func detectCompressionFromURL(imageURL string) Compression {
	switch filepath.Ext(imageURL) {
	case ".bzip2":
		return CompressionBzip2
	case ".gz":
		return CompressionGzip
	case ".xz":
		return CompressionXZ
	case ".zstd", ".zst", ".zs":
		return CompressionZstd
	default:
		return CompressionNone
	}
}

// Write will pull an image and write it to local storage device.
// If compression is not CompressionNone, it will decompress the data before
// writing to the underlying device.
func Write(sourceImage, destinationDevice string, compression Compression) error {
	ctx := context.Background()
	client := http.DefaultClient
	opts := docker.ResolverOptions{}
	client.Transport = &http.Transport{
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: true, //nolint:gosec // GA402 TODO
		},
	}

	opts.Client = client

	resolver := docker.NewResolver(opts)

	fileOut, err := os.OpenFile(destinationDevice, os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer fileOut.Close()
	customMediaType := "application/vnd.oci.image.layer.v1.tar"
	allowedMediaTypes := []string{customMediaType}

	f := NewDiskImageStore(compression, fileOut)

	log.Infof("Beginning write of image [%s] to disk [%s]", filepath.Base(sourceImage), destinationDevice)
	pullOpts := []oras.PullOpt{
		oras.WithAllowedMediaTypes(allowedMediaTypes),
		oras.WithPullStatusTrack(os.Stdout),
	}
	_, _, err = oras.Pull(ctx, resolver, sourceImage, f, pullOpts...)
	if err != nil {
		if errors.Is(err, reference.ErrObjectRequired) {
			return fmt.Errorf("image reference format is invalid. Please specify <name:tag|name@digest>")
		}
		return err
	}

	// Do the equivalent of partprobe on the device
	if err := fileOut.Sync(); err != nil {
		log.Warnf("Failed to sync the block device")
	}

	if err := unix.IoctlSetInt(int(fileOut.Fd()), unix.BLKRRPART, 0); err != nil {
		log.Warnf("Error re-probing the partitions for the specified device")
	}

	return nil
}

func newDecompressor(compression Compression, r io.Reader) (io.ReadCloser, error) {
	switch compression {
	case CompressionBzip2:
		return io.NopCloser(bzip2.NewReader(r)), nil
	case CompressionGzip:
		reader, err := gzip.NewReader(r)
		if err != nil {
			return nil, fmt.Errorf("[ERROR] New gzip reader: %w", err)
		}
		return reader, nil
	case CompressionXZ:
		reader, err := xz.NewReader(r)
		if err != nil {
			return nil, fmt.Errorf("[ERROR] New xz reader: %w", err)
		}
		return io.NopCloser(reader), nil
	case CompressionZstd:
		reader, err := zstd.NewReader(r)
		if err != nil {
			return nil, fmt.Errorf("[ERROR] New zs reader: %w", err)
		}
		return reader.IOReadCloser(), nil
	case CompressionNone:
		return io.NopCloser(r), nil
	}

	return nil, fmt.Errorf("[ERROR] Unknown compression type: %d", compression)
}
