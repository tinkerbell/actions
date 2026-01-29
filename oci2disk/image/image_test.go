package image

import (
	"bytes"
	"compress/gzip"
	"io"
	"strings"
	"testing"

	"github.com/ulikunitz/xz"
)

func gzipReader(t *testing.T) io.Reader {
	t.Helper()

	var b bytes.Buffer
	gzW := gzip.NewWriter(&b)
	if _, err := gzW.Write([]byte("YourDataHere")); err != nil {
		t.Fatal(err)
	}
	if err := gzW.Close(); err != nil {
		t.Fatal(err)
	}
	rdata := strings.NewReader(b.String())

	return rdata
}

func xzReader(t *testing.T) io.Reader {
	t.Helper()

	var b bytes.Buffer
	xzW, _ := xz.NewWriter(&b)
	if _, err := xzW.Write([]byte("YourDataHere")); err != nil {
		t.Fatal(err)
	}
	if err := xzW.Close(); err != nil {
		t.Fatal(err)
	}
	rdata := strings.NewReader(b.String())

	return rdata
}

func rawReader(t *testing.T) io.Reader {
	t.Helper()
	return strings.NewReader("YourDataHere")
}

func Test_newDecompressor(t *testing.T) {
	tests := []struct {
		name        string
		compression Compression
		reader      func(*testing.T) io.Reader
		wantErr     bool
	}{
		{
			"gzip",
			CompressionGzip,
			gzipReader,
			false,
		},
		{
			"broken gzip",
			CompressionGzip,
			xzReader,
			true,
		},
		{
			"xz",
			CompressionXZ,
			xzReader,
			false,
		},
		{
			"none",
			CompressionNone,
			rawReader,
			false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := newDecompressor(tt.compression, tt.reader(t))
			if (err != nil) != tt.wantErr {
				t.Errorf("newDecompressor() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
		})
	}
}

func TestParseCompression(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		imageURL string
		expected Compression
	}{
		{"gz", "gz", "", CompressionGzip},
		{"gzip", "gzip", "", CompressionGzip},
		{"xz", "xz", "", CompressionXZ},
		{"zstd", "zstd", "", CompressionZstd},
		{"zst", "zst", "", CompressionZstd},
		{"zs", "zs", "", CompressionZstd},
		{"bzip2", "bzip2", "", CompressionBzip2},
		{"empty", "", "", CompressionNone},
		{"false", "false", "", CompressionNone},
		{"unknown", "unknown", "", CompressionNone},
		// Backward compatibility: true with URL detection
		{"true with .gz URL", "true", "registry/image:tag.gz", CompressionGzip},
		{"true with .gzip URL", "true", "registry/image:tag.gzip", CompressionGzip},
		{"true with .xz URL", "true", "registry/image:tag.xz", CompressionXZ},
		{"true with .zstd URL", "true", "registry/image:tag.zstd", CompressionZstd},
		{"true with .zst URL", "true", "registry/image:tag.zst", CompressionZstd},
		{"true with .zs URL", "true", "registry/image:tag.zs", CompressionZstd},
		{"true with .bzip2 URL", "true", "registry/image:tag.bzip2", CompressionBzip2},
		{"true with no extension", "true", "registry/image:latest", CompressionNone},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ParseCompression(tt.input, tt.imageURL)
			if got != tt.expected {
				t.Errorf("ParseCompression(%q, %q) = %v, want %v", tt.input, tt.imageURL, got, tt.expected)
			}
		})
	}
}
