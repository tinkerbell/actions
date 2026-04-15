package archive

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// buildTarGz builds a small in-memory gzipped tar archive containing a single
// regular file and returns the compressed bytes.
func buildTarGz(t *testing.T) []byte {
	t.Helper()

	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)

	content := []byte("hello archive2disk\n")
	hdr := &tar.Header{
		Name:     "hello.txt",
		Mode:     0o644,
		Size:     int64(len(content)),
		Typeflag: tar.TypeReg,
	}
	if err := tw.WriteHeader(hdr); err != nil {
		t.Fatalf("write tar header: %v", err)
	}
	if _, err := tw.Write(content); err != nil {
		t.Fatalf("write tar content: %v", err)
	}
	if err := tw.Close(); err != nil {
		t.Fatalf("close tar writer: %v", err)
	}
	if err := gz.Close(); err != nil {
		t.Fatalf("close gzip writer: %v", err)
	}

	return buf.Bytes()
}

// newArchiveServer returns an httptest server that serves the given bytes at /archive.tar.gz.
func newArchiveServer(t *testing.T, body []byte) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/gzip")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(body)
	}))
}

func sha256Hex(b []byte) string {
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

func TestWrite_CorrectChecksum_Succeeds(t *testing.T) {
	archiveBytes := buildTarGz(t)
	srv := newArchiveServer(t, archiveBytes)
	defer srv.Close()

	dest := t.TempDir()
	checksum := "sha256:" + sha256Hex(archiveBytes)

	if err := Write(srv.URL+"/archive.tar.gz", "targz", dest, checksum, 1); err != nil {
		t.Fatalf("expected success, got error: %v", err)
	}

	got, err := os.ReadFile(filepath.Join(dest, "hello.txt"))
	if err != nil {
		t.Fatalf("expected extracted file to exist: %v", err)
	}
	if string(got) != "hello archive2disk\n" {
		t.Fatalf("unexpected extracted content: %q", string(got))
	}
}

func TestWrite_WrongChecksum_ReturnsDigestMismatch(t *testing.T) {
	archiveBytes := buildTarGz(t)
	srv := newArchiveServer(t, archiveBytes)
	defer srv.Close()

	dest := t.TempDir()
	// A syntactically valid but incorrect sha256.
	wrong := "sha256:" + strings.Repeat("0", 64)

	err := Write(srv.URL+"/archive.tar.gz", "targz", dest, wrong, 1)
	if err == nil {
		t.Fatalf("expected digest mismatch error, got nil")
	}
	if !strings.Contains(err.Error(), "digest mismatch") {
		t.Fatalf("expected error containing 'digest mismatch', got: %v", err)
	}
}

func TestWrite_EmptyChecksum_SkipsVerification(t *testing.T) {
	archiveBytes := buildTarGz(t)
	srv := newArchiveServer(t, archiveBytes)
	defer srv.Close()

	dest := t.TempDir()

	if err := Write(srv.URL+"/archive.tar.gz", "targz", dest, "", 1); err != nil {
		t.Fatalf("expected success with no checksum, got error: %v", err)
	}

	if _, err := os.Stat(filepath.Join(dest, "hello.txt")); err != nil {
		t.Fatalf("expected extracted file to exist: %v", err)
	}
}

func TestWrite_MalformedChecksum_ReturnsParseError(t *testing.T) {
	archiveBytes := buildTarGz(t)
	srv := newArchiveServer(t, archiveBytes)
	defer srv.Close()

	dest := t.TempDir()

	err := Write(srv.URL+"/archive.tar.gz", "targz", dest, "not-a-valid-digest", 1)
	if err == nil {
		t.Fatalf("expected parse error, got nil")
	}
	if !strings.Contains(err.Error(), "failed to parse digest") {
		t.Fatalf("expected error containing 'failed to parse digest', got: %v", err)
	}
}

// TestWrite_ChecksumMatchesSha256OfHTTPBody is a belt-and-braces check that the
// checksum users are expected to pass is literally `sha256sum file.tar.gz` of
// the bytes on the wire, not the hash of decompressed or tar-framed content.
func TestWrite_ChecksumMatchesSha256OfHTTPBody(t *testing.T) {
	archiveBytes := buildTarGz(t)

	// Compute what a user would get from `sha256sum` on the compressed file.
	sum := sha256.Sum256(archiveBytes)
	expected := fmt.Sprintf("sha256:%x", sum)

	srv := newArchiveServer(t, archiveBytes)
	defer srv.Close()

	dest := t.TempDir()
	if err := Write(srv.URL+"/archive.tar.gz", "targz", dest, expected, 1); err != nil {
		t.Fatalf("expected the sha256 of the raw HTTP body to verify, got error: %v", err)
	}
}
