package image

// This package handles the pulling and management of images

import (
	"compress/bzip2"
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"runtime"
	"sync/atomic"
	"time"

	"github.com/klauspost/compress/zstd"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	log "github.com/sirupsen/logrus"
	"github.com/ulikunitz/xz"
	"golang.org/x/sys/unix"
	"oras.land/oras-go/v2"
	"oras.land/oras-go/v2/content"
	"oras.land/oras-go/v2/registry/remote"
	"oras.land/oras-go/v2/registry/remote/auth"
)

// BLKRRPART is the ioctl request to re-read partition table (Linux-specific)
const BLKRRPART = 0x125f

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
	p.wBytes.Add(int64(nu))
	return nu, err
}

func (p *Progress) Read(b []byte) (n int, err error) {
	nu, err := p.r.Read(b)
	p.rBytes.Add(int64(nu))
	return nu, err
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

// Write will pull an image and write it to local storage device
// Compression type is automatically detected from the layer's org.opencontainers.image.title annotation.
// Platform is automatically detected from the runtime (linux/arm64, linux/amd64, etc.).
func Write(sourceImage, destinationDevice string) error {
	ctx := context.Background()

	// Detect platform from runtime
	platformOS := "linux"
	platformArch := runtime.GOARCH

	// Create repository connection
	repo, err := remote.NewRepository(sourceImage)
	if err != nil {
		return fmt.Errorf("failed to create repository: %w", err)
	}

	// Configure repository client with custom HTTP client
	repo.Client = auth.DefaultClient

	// Open destination device
	fileOut, err := os.OpenFile(destinationDevice, os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer fileOut.Close()

	log.Infof("Beginning write of image [%s] to disk [%s]", filepath.Base(sourceImage), destinationDevice)

	// Resolve the manifest descriptor with optional platform filtering
	tagOrDigest := repo.Reference.Reference
	if tagOrDigest == "" {
		return fmt.Errorf("image reference format is invalid. Please specify <name:tag|name@digest>")
	}

	// Configure resolution options with platform filtering if specified
	resolveOpts := oras.ResolveOptions{}
	if platformOS != "" && platformArch != "" {
		resolveOpts.TargetPlatform = &ocispec.Platform{
			OS:           platformOS,
			Architecture: platformArch,
		}
		log.Infof("Filtering for platform: %s/%s", platformOS, platformArch)
	}

	manifestDescriptor, err := oras.Resolve(ctx, repo, tagOrDigest, resolveOpts)
	if err != nil {
		return fmt.Errorf("failed to resolve manifest: %w", err)
	}

	// Fetch the manifest
	manifestBytes, err := content.FetchAll(ctx, repo, manifestDescriptor)
	if err != nil {
		return fmt.Errorf("failed to fetch manifest: %w", err)
	}

	// Parse the manifest
	var manifest ocispec.Manifest
	if err := json.Unmarshal(manifestBytes, &manifest); err != nil {
		return fmt.Errorf("failed to parse manifest: %w", err)
	}

	// Filter and process layers
	customMediaType := "application/vnd.oci.image.layer.v1.tar"
	var totalBytes int64
	var processedLayers int

	for _, layer := range manifest.Layers {
		// Skip layers that don't match our media type
		if layer.MediaType != customMediaType {
			log.Debugf("Skipping layer with media type: %s", layer.MediaType)
			continue
		}

		log.Infof("Fetching layer: %s (size: %d bytes)", layer.Digest, layer.Size)

		// Fetch the layer content
		layerReader, err := repo.Fetch(ctx, layer)
		if err != nil {
			return fmt.Errorf("failed to fetch layer %s: %w", layer.Digest, err)
		}

		// Determine compression and create appropriate reader
		var sourceReader io.Reader = layerReader
		var decompressor io.ReadCloser

		// Determine compression type from annotation
		if layer.Annotations != nil {
			if titleAnnotation, ok := layer.Annotations[ocispec.AnnotationTitle]; ok {
				// Create decompressor if needed
				var err error
				decompressor, err = findDecompressor(titleAnnotation, layerReader)
				if err != nil {
					layerReader.Close()
					return fmt.Errorf("failed to create decompressor: %w", err)
				}
				sourceReader = decompressor
			}
		}

		// Create progress tracker that wraps the writer and source reader
		progress := NewProgress(fileOut, sourceReader)

		// Start progress reporting
		ticker := time.NewTicker(5 * time.Second)
		done := make(chan bool)
		go func() {
			layerSize := layer.Size
			for {
				select {
				case <-done:
					log.Infof("Progress: written=%s, compressed=%s, read=%s",
						prettyByteSize(progress.writeBytes()),
						prettyByteSize(layerSize),
						prettyByteSize(progress.readBytes()))
					return
				case <-ticker.C:
					log.Infof("Progress: written=%s, compressed=%s, read=%s",
						prettyByteSize(progress.writeBytes()),
						prettyByteSize(layerSize),
						prettyByteSize(progress.readBytes()))
				}
			}
		}()

		// Copy to destination device: read from progress (which reads from sourceReader) and write to progress
		_, err = io.Copy(progress, progress)

		// Stop progress reporting
		ticker.Stop()
		done <- true
		
		// Close in correct order: decompressor first (if exists), then underlying layerReader
		if decompressor != nil {
			decompressor.Close()
		}
		layerReader.Close()

		if err != nil {
			return fmt.Errorf("failed to write layer to device: %w", err)
		}

		totalBytes += progress.writeBytes()
		processedLayers++
	}

	if processedLayers == 0 {
		return fmt.Errorf("no layers with media type %s found in manifest", customMediaType)
	}

	log.Infof("Successfully processed %d layer(s), total: %s", processedLayers, prettyByteSize(totalBytes))

	// Do the equivalent of partprobe on the device
	if err := fileOut.Sync(); err != nil {
		log.Warnf("Failed to sync the block device")
	}

	if err := unix.IoctlSetInt(int(fileOut.Fd()), BLKRRPART, 0); err != nil {
		log.Warnf("Error re-probing the partitions for the specified device")
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
