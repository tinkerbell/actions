package image

import (
	"bytes"
	"compress/gzip"
	"io"
	"log"
	"strings"
	"testing"

	"github.com/ulikunitz/xz"
)

func gzipReader() io.Reader {
	var b bytes.Buffer
	gzW := gzip.NewWriter(&b)
	if _, err := gzW.Write([]byte("YourDataHere")); err != nil {
		log.Fatal(err)
	}
	if err := gzW.Close(); err != nil {
		log.Fatal(err)
	}
	rdata := strings.NewReader(b.String())

	return rdata
}

func xzReader() io.Reader {
	var b bytes.Buffer
	xzW, _ := xz.NewWriter(&b)
	if _, err := xzW.Write([]byte("YourDataHere")); err != nil {
		log.Fatal(err)
	}
	if err := xzW.Close(); err != nil {
		log.Fatal(err)
	}
	rdata := strings.NewReader(b.String())

	return rdata
}

func Test_findDecompressor(t *testing.T) {
	type args struct {
		imageURL string
		r        io.Reader
	}
	tests := []struct {
		name    string
		args    args
		wantOut io.Reader
		wantErr bool
	}{
		{
			"tar gzip",
			args{imageURL: "http://192.168.0.1/a.tar.gz", r: gzipReader()},
			nil,
			false,
		},
		{
			"broken gzip",
			args{imageURL: "http://192.168.0.1/a.gz", r: xzReader()},
			nil,
			true,
		},
		{
			"xz",
			args{imageURL: "http://192.168.0.1/a.xz", r: xzReader()},
			nil,
			false,
		},
		{
			"unknown",
			args{imageURL: "http://192.168.0.1/a.abc", r: xzReader()},
			nil,
			true,
		},
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			_, err := findDecompressor(tt.args.imageURL, tt.args.r)
			if (err != nil) != tt.wantErr {
				t.Errorf("findDecompressor() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
		})
	}
}
