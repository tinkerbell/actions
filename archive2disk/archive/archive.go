package archive

// This package handles the pulling and management of images

import (
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	// See https://github.com/opencontainers/go-digest#usage
	_ "crypto/sha256"
	_ "crypto/sha512"

	digest "github.com/opencontainers/go-digest"
	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
)

// Write will pull an image and write it to local storage device image must be a tar file or tar.gz file as set by archiveType.
// When checksum is non-empty it must be of the form "<algorithm>:<hash>" (e.g. sha256:...) and is verified against
// the raw HTTP response body (i.e. the bytes of the downloaded archive as produced by `sha256sum file.tar.gz`).
func Write(archiveURL, archiveType, path string, checksum string, httpTimeoutVal int) error {
	req, err := http.NewRequest("GET", archiveURL, nil)
	if err != nil {
		return err
	}

	timeout := time.Duration(httpTimeoutVal) * time.Minute
	client := &http.Client{Timeout: timeout}
	log.Infof("httpTimeoutVal [%d]", timeout)
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode > 300 {
		// Customise response for the 404 to make degugging simpler
		if resp.StatusCode == 404 {
			return fmt.Errorf("%s not found", archiveURL)
		}
		return fmt.Errorf("%s", resp.Status)
	}

	absPath, err := filepath.Abs(path)
	if err != nil {
		return fmt.Errorf("failed to get absolute path: %w", err)
	}

	// Wrap the raw HTTP body with a TeeReader so the verifier hashes the full
	// downloaded archive (what users get from `sha256sum file.tar.gz`), not just
	// the bytes consumed by tar/gzip parsers.
	var verifier digest.Verifier
	var reader io.Reader = resp.Body
	if checksum != "" {
		log.Infof("checksum validation enabled [%s]", checksum)
		dd, err := digest.Parse(checksum)
		if err != nil {
			return fmt.Errorf("failed to parse digest - %w", err)
		}
		verifier = dd.Verifier()
		reader = io.TeeReader(resp.Body, verifier)
	}

	switch strings.ToLower(archiveType) {
	case "tar":
		if err := extractTarDirectory(absPath, reader); err != nil {
			return fmt.Errorf("[ERROR] New gzip reader: %w", err)
		}
	case "targz":
		if err := extractTarGzip(absPath, reader); err != nil {
			return fmt.Errorf("[ERROR] New gzip reader: %w", err)
		}
	default:
		return fmt.Errorf("[ERROR] Unknown archiveType supplied: %v", archiveType)
	}

	if verifier != nil {
		// Drain any remaining bytes (gzip footer, tar padding past end-of-archive
		// marker, etc.) so the verifier sees the entire HTTP body.
		if _, err := io.Copy(io.Discard, reader); err != nil {
			return fmt.Errorf("failed to drain response body for checksum verification: %w", err)
		}
		if !verifier.Verified() {
			return errors.New("digest mismatch")
		}
	}

	return nil
}
