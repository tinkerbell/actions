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

func Test_findDecompressor(t *testing.T) {
	tests := []struct {
		name     string
		filename string
		reader   func(*testing.T) io.Reader
		wantOut  io.Reader
		wantErr  bool
	}{
		{
			"gzip",
			"image.raw.gz",
			gzipReader,
			nil,
			false,
		},
		{
			"broken gzip",
			"image.raw.gz",
			xzReader,
			nil,
			true,
		},
		{
			"xz",
			"image.raw.xz",
			xzReader,
			nil,
			false,
		},
		{
			"unknown extension",
			"image.raw.abc",
			xzReader,
			nil,
			true,
		},
		{
			"no extension",
			"image.raw",
			gzipReader,
			nil,
			true,
		},
		{
			"bzip2",
			"image.raw.bz2",
			gzipReader,
			nil,
			false,
		},
		{
			"zstd .zst",
			"image.raw.zst",
			gzipReader,
			nil,
			false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := findDecompressor(tt.filename, tt.reader(t))
			if (err != nil) != tt.wantErr {
				t.Errorf("findDecompressor() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
		})
	}
}
