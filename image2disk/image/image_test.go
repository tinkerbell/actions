package image

import (
	"bytes"
	"context"
	"io"
	"strings"
	"testing"

	kgzip "github.com/klauspost/compress/gzip"
	"github.com/klauspost/compress/zstd"
	"github.com/ulikunitz/xz"
)

func gzipReader(t *testing.T) io.Reader {
	t.Helper()

	var b bytes.Buffer
	gzW := kgzip.NewWriter(&b)
	if _, err := gzW.Write([]byte("YourDataHere")); err != nil {
		t.Fatal(err)
	}
	if err := gzW.Close(); err != nil {
		t.Fatal(err)
	}

	return strings.NewReader(b.String())
}

func xzReader(t *testing.T) io.Reader {
	t.Helper()

	var b bytes.Buffer
	xzW, err := xz.NewWriter(&b)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := xzW.Write([]byte("YourDataHere")); err != nil {
		t.Fatal(err)
	}
	if err := xzW.Close(); err != nil {
		t.Fatal(err)
	}

	return strings.NewReader(b.String())
}

func zstdReader(t *testing.T) io.Reader {
	t.Helper()

	var b bytes.Buffer
	zw, err := zstd.NewWriter(&b)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := zw.Write([]byte("YourDataHere")); err != nil {
		t.Fatal(err)
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}

	return strings.NewReader(b.String())
}

func Test_findDecompressor(t *testing.T) {
	tests := []struct {
		name       string
		imageURL   string
		reader     func(*testing.T) io.Reader
		wantFormat string
		wantErr    bool
	}{
		{"tar gzip", "http://192.168.0.1/a.tar.gz", gzipReader, formatGzip, false},
		{"broken gzip", "http://192.168.0.1/a.gz", xzReader, formatGzip, true},
		{"xz", "http://192.168.0.1/a.xz", xzReader, formatXZ, false},
		{"zstd", "http://192.168.0.1/a.zst", zstdReader, formatZstd, false},
		{"unknown", "http://192.168.0.1/a.abc", xzReader, "", true},
		// A presigned URL carries a query string. filepath.Ext on the raw URL
		// yields ".gz?token=abc" and finds nothing.
		{"gzip behind a query string", "http://192.168.0.1/a.raw.gz?token=abc&x=1", gzipReader, formatGzip, false},
		{"zstd behind a query string", "https://host/img.raw.zst?X-Amz-Signature=deadbeef", zstdReader, formatZstd, false},
		{"extension in the query only", "http://192.168.0.1/download?file=a.gz", gzipReader, "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rc, format, err := findDecompressor(context.Background(), tt.imageURL, tt.reader(t))
			if (err != nil) != tt.wantErr {
				t.Fatalf("findDecompressor() error = %v, wantErr %v", err, tt.wantErr)
			}
			if format != tt.wantFormat {
				t.Errorf("format = %q, want %q", format, tt.wantFormat)
			}
			if err != nil {
				return
			}
			defer rc.Close()

			got, err := io.ReadAll(rc)
			if err != nil {
				t.Fatalf("reading decompressed stream: %v", err)
			}
			if string(got) != "YourDataHere" {
				t.Errorf("decompressed to %q, want %q", got, "YourDataHere")
			}
		})
	}
}

func Test_imageExt(t *testing.T) {
	for _, tt := range []struct {
		url  string
		want string
	}{
		{"http://host/a.raw.gz", ".gz"},
		{"http://host/a.raw.gz?token=abc", ".gz"},
		{"https://host/path/img.zst?X-Amz-Signature=x&Y=2", ".zst"},
		{"http://host/a.raw.gz#frag", ".gz"},
		{"http://host/noext", ""},
		{"/local/path/a.xz", ".xz"},
	} {
		t.Run(tt.url, func(t *testing.T) {
			if got := imageExt(tt.url); got != tt.want {
				t.Errorf("imageExt(%q) = %q, want %q", tt.url, got, tt.want)
			}
		})
	}
}
