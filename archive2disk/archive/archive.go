package archive

// This package handles the pulling and management of images

import (
	"fmt"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	log "github.com/sirupsen/logrus"
)

// Write will pull an image and write it to local storage device image must be a tar file or tar.gz file as set by archiveType.
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

	switch strings.ToLower(archiveType) {
	case "tar":
		err := extractTarDirectory(absPath, checksum, resp.Body)
		if err != nil {
			return fmt.Errorf("[ERROR] New gzip reader: %w", err)
		}
	case "targz":
		err := extractTarGzip(absPath, checksum, resp.Body)
		if err != nil {
			return fmt.Errorf("[ERROR] New gzip reader: %w", err)
		}
	default:
		return fmt.Errorf("[ERROR] Unknown archiveType supplied: %v", archiveType)
	}
	return nil
}
