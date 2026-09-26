package image

import (
	"context"
	"fmt"
	"io"
	"net/url"
	"path"
	"runtime"

	"github.com/cosnicolaou/pbzip2"
	kgzip "github.com/klauspost/compress/gzip"
	"github.com/klauspost/compress/zstd"
	"github.com/ulikunitz/xz"
)

// Names of the supported compression formats, as reported in Result.Format.
const (
	formatNone  = "none"
	formatGzip  = "gzip"
	formatZstd  = "zstd"
	formatXZ    = "xz"
	formatBzip2 = "bzip2"
)

// imageExt returns the file extension of an image URL's path.
//
// filepath.Ext on the raw URL is wrong: a presigned S3 or GCS link such as
// https://host/img.raw.gz?X-Amz-Signature=... yields ".gz?X-Amz-Signature=..."
// and no format matches.
func imageExt(imageURL string) string {
	u, err := url.Parse(imageURL)
	if err != nil {
		return path.Ext(imageURL)
	}
	return path.Ext(u.Path)
}

// findDecompressor returns a decoder for the compression format named by the
// image URL's extension, wrapping r. The caller must Close the result.
func findDecompressor(ctx context.Context, imageURL string, r io.Reader) (io.ReadCloser, string, error) {
	switch imageExt(imageURL) {
	case ".bzip2", ".bz2":
		// bzip2 blocks are independent, so any .bz2 decodes across all cores.
		rd := pbzip2.NewReader(ctx, r,
			pbzip2.DecompressionOptions(pbzip2.BZConcurrency(runtime.GOMAXPROCS(0))),
		)
		return io.NopCloser(rd), formatBzip2, nil

	case ".gz":
		// klauspost's inflate is faster than the standard library's, and is
		// already a dependency. DEFLATE is a single sliding window, so this is
		// strictly serial however many cores are available.
		rd, err := kgzip.NewReader(r)
		if err != nil {
			return nil, formatGzip, fmt.Errorf("new gzip reader: %w", err)
		}
		return rd, formatGzip, nil

	case ".xz":
		rd, err := xz.NewReader(r)
		if err != nil {
			return nil, formatXZ, fmt.Errorf("new xz reader: %w", err)
		}
		return io.NopCloser(rd), formatXZ, nil

	case ".zs", ".zst":
		rd, err := zstd.NewReader(r,
			// The default is min(4, GOMAXPROCS). klauspost's stream decoder
			// saturates around three cores whatever this is, so raising it is a
			// bounded win; the cap keeps decoder window memory in hand.
			zstd.WithDecoderConcurrency(zstdConcurrency()),
			zstd.WithDecoderLowmem(false),
			// Bound the window a hostile or --long=31 image could ask for.
			zstd.WithDecoderMaxWindow(1<<30),
		)
		if err != nil {
			return nil, formatZstd, fmt.Errorf("new zstd reader: %w", err)
		}
		// Close is mandatory: it joins the decoder's internal goroutines.
		return rd.IOReadCloser(), formatZstd, nil
	}

	return nil, "", fmt.Errorf("unknown compression suffix [%s]", imageExt(imageURL))
}
