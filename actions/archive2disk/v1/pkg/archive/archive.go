package archive

// This package handles the pulling and management of images

import (
	"archive/tar"
	"compress/gzip"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"

	log "github.com/sirupsen/logrus"
)

// Write will pull an image and write it to local storage device
// with compress set to true it will use gzip compression to expand the data before
// writing to an underlying device
func Write(archiveURL, archiveType, path string) error {

	req, err := http.NewRequest("GET", archiveURL, nil)
	if err != nil {
		return err
	}

	resp, err := http.DefaultClient.Do(req)
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

	var out *tar.Reader
	absPath, err := filepath.Abs(path)

	switch archiveType {
	case "gz":
		// With compression run data through gzip writer
		// zipOUT, err := gzip.NewReader(resp.Body)
		// if err != nil {
		// 	fmt.Println("[ERROR] New gzip reader:", err)
		// }
		// defer zipOUT.Close()
		// out = zipOUT
	case "tar":
		// With compression run data through gzip writer
		out = tar.NewReader(resp.Body)
		//out = tarOUT
	case "targz":
		// With compression run data through gzip writer
		zipOUT, err := gzip.NewReader(resp.Body)
		if err != nil {
			log.Fatalf("[ERROR] New gzip reader:", err)
		}
		defer zipOUT.Close()
		out = tar.NewReader(zipOUT)
		//out = tarOUT
	default:

	}
	// untar each segment
	for {
		hdr, err := out.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
		// determine proper file path info
		finfo := hdr.FileInfo()
		fileName := hdr.Name
		absFileName := filepath.Join(absPath, fileName)
		// if a dir, create it, then go to next segment
		if finfo.Mode().IsDir() {
			if err := os.MkdirAll(absFileName, 0755); err != nil {
				return err
			}
			continue
		}
		// create new file with original file mode
		file, err := os.OpenFile(
			absFileName,
			os.O_RDWR|os.O_CREATE|os.O_TRUNC,
			finfo.Mode().Perm(),
		)
		if err != nil {
			return err
		}
		n, cpErr := io.Copy(file, out)
		if closeErr := file.Close(); closeErr != nil {
			return err
		}
		if cpErr != nil {
			return cpErr
		}
		if n != finfo.Size() {
			return fmt.Errorf("wrote %d, want %d", n, finfo.Size())
		}
		log.Infof("Extracted [%s] to [%s]", fileName, absFileName)
	}
	return nil
}
