package image

// This package handles the pulling and management of images

import (
	"compress/bzip2"
	"compress/gzip"
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/containerd/containerd/reference"
	"github.com/containerd/containerd/remotes/docker"
	"github.com/deislabs/oras/pkg/oras"

	"github.com/dustin/go-humanize"
	"github.com/klauspost/compress/zstd"
	log "github.com/sirupsen/logrus"
	"github.com/ulikunitz/xz"
	"golang.org/x/sys/unix"
)

var tick chan time.Time

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
// writing to an underlying device
func Write(sourceImage, destinationDevice string, compressed bool) error {
	ctx := context.Background()
	client := http.DefaultClient
	opts := docker.ResolverOptions{}
	client.Transport = &http.Transport{
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: true,
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

	f := NewDiskImageStore(sourceImage, compressed, fileOut)

	log.Infof("Beginning write of image [%s] to disk [%s]", filepath.Base(sourceImage), destinationDevice)
	pullOpts := []oras.PullOpt{
		oras.WithAllowedMediaTypes(allowedMediaTypes),
		oras.WithPullStatusTrack(os.Stdout),
	}
	_, _, err = oras.Pull(ctx, resolver, sourceImage, f, pullOpts...)
	if err != nil {
		if err == reference.ErrObjectRequired {
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

func findDecompressor(imageURL string, r io.Reader) (out io.Reader, err error) {
	switch filepath.Ext(imageURL) {
	case ".bzip2":
		// With compression run data through gzip writer
		bzipOUT := bzip2.NewReader(r)
		out = bzipOUT
	case ".gz":
		// With compression run data through gzip writer
		zipOUT, gzErr := gzip.NewReader(r)
		if gzErr != nil {
			err = fmt.Errorf("[ERROR] New gzip reader: %v", gzErr)
			return
		}
		defer zipOUT.Close()
		out = zipOUT
	case ".xz":
		xzOUT, xzErr := xz.NewReader(r)
		if xzErr != nil {
			err = fmt.Errorf("[ERROR] New xz reader: %v", xzErr)
			return
		}
		// The xz reader doesn't implement close()
		// defer xzOUT.Close()
		out = xzOUT
	case ".zs":
		zsOUT, zsErr := zstd.NewReader(r)
		if zsErr != nil {
			err = fmt.Errorf("[ERROR] New zs reader: %v", zsErr)
			return
		}
		defer zsOUT.Close()
		out = zsOUT
	default:
		err = fmt.Errorf("Unknown compression suffix [%s]", filepath.Ext(imageURL))
	}
	return
}
