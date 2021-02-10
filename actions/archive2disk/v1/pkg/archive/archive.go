package archive

// This package handles the pulling and management of images

import (
	"archive/tar"
	"compress/gzip"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/dustin/go-humanize"
)

var tick chan time.Time

// WriteCounter counts the number of bytes written to it. It implements to the io.Writer interface
// and we can pass this into io.TeeReader() which will report progress on each write cycle.
type WriteCounter struct {
	Total uint64
}

func (wc *WriteCounter) Write(p []byte) (int, error) {
	n := len(p)
	wc.Total += uint64(n)
	return n, nil
}

func tickerProgress(byteCounter uint64) {
	// Clear the line by using a character return to go back to the start and remove
	// the remaining characters by filling it with spaces
	fmt.Printf("\r%s", strings.Repeat(" ", 35))

	// Return again and print current status of download
	// We use the humanize package to print the bytes in a meaningful way (e.g. 10 MB)
	fmt.Printf("\rDownloading... %s complete", humanize.Bytes(byteCounter))
}

// Read - will take a local disk and copy an image to a remote server
func Read(sourceDevice, destinationAddress, mac string, compressed bool) error {

	var fileName string
	if !compressed {
		// raw image
		fileName = fmt.Sprintf("%s.img", mac)
	} else {
		// compressed image
		fileName = fmt.Sprintf("%s.zmg", mac)
	}

	fmt.Println("--------------------------------------------------------------------------------")

	fmt.Printf("\nReading of disk [%s], and sending to [%s]\n", filepath.Base(sourceDevice), destinationAddress)
	fmt.Println("--------------------------------------------------------------------------------")

	client := &http.Client{}
	_, err := UploadMultipartFile(client, destinationAddress, fileName, sourceDevice, compressed)
	if err != nil {
		return err
	}

	return nil
}

//UploadMultipartFile -
func UploadMultipartFile(client *http.Client, uri, key, path string, compressed bool) (*http.Response, error) {
	body, writer := io.Pipe()

	req, err := http.NewRequest(http.MethodPost, uri, body)
	if err != nil {
		return nil, err
	}

	mwriter := multipart.NewWriter(writer)
	req.Header.Add("Content-Type", mwriter.FormDataContentType())

	errchan := make(chan error)

	// GO routine for the copy operation
	go func() {
		defer close(errchan)
		defer writer.Close()
		defer mwriter.Close()

		// BootyImage is the key that the client will lookfor and
		// key is the renamed file
		w, err := mwriter.CreateFormFile("BootyImage", key)
		if err != nil {
			errchan <- err
			return
		}

		diskIn, err := os.Open(path)
		if err != nil {
			errchan <- err
			return
		}

		defer diskIn.Close()

		if !compressed {
			// Without compression read raw output
			if written, err := io.Copy(w, diskIn); err != nil {
				errchan <- fmt.Errorf("error copying %s (%d bytes written): %v", path, written, err)
				return
			}

		} else {
			// With compression run data through gzip writer
			zipWriter := gzip.NewWriter(w)
			if err != nil {
				errchan <- fmt.Errorf("[ERROR] New gzip reader: %s", err)
				return
			}

			// run an io.Copy on the disk into the zipWriter
			if written, err := io.Copy(zipWriter, diskIn); err != nil {
				errchan <- fmt.Errorf("error copying %s (%d bytes written): %v", path, written, err)
				return
			}
			// Ensure we close our zipWriter (otherwise we will get "unexpected EOF")
			err = zipWriter.Close()

		}

		if err := mwriter.Close(); err != nil {
			errchan <- err
			return
		}

	}()

	resp, err := client.Do(req)
	merr := <-errchan

	if err != nil || merr != nil {
		return resp, fmt.Errorf("http error: %v, multipart error: %v", err, merr)
	}

	return resp, nil
}

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
			fmt.Println("[ERROR] New gzip reader:", err)
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
		fmt.Printf("x %s\n", absFileName)
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
	}
	// fileOut, err := os.OpenFile(destinationDevice, os.O_CREATE|os.O_WRONLY, 0644)
	// if err != nil {
	// 	return err
	// }
	// defer fileOut.Close()

	// if !compressed {
	// 	// Without compression send raw output
	// 	out = resp.Body
	// } else {
	// 	// With compression run data through gzip writer
	// 	zipOUT, err := gzip.NewReader(resp.Body)
	// 	if err != nil {
	// 		fmt.Println("[ERROR] New gzip reader:", err)
	// 	}
	// 	defer zipOUT.Close()
	// 	out = zipOUT
	// }

	// log.Infof("Beginning write of image [%s] to disk [%s]", filepath.Base(sourceImage), destinationDevice)
	// // Create our progress reporter and pass it to be used alongside our writer
	// ticker := time.NewTicker(500 * time.Millisecond)
	// counter := &WriteCounter{}

	// go func() {
	// 	for ; true; <-ticker.C {
	// 		tickerProgress(counter.Total)
	// 	}
	// }()
	// if _, err = io.Copy(fileOut, io.TeeReader(out, counter)); err != nil {
	// 	ticker.Stop()
	// 	return err
	// }

	// count, err := io.Copy(fileOut, out)
	// if err != nil {
	// 	ticker.Stop()
	// 	return fmt.Errorf("Error writing %d bytes to disk [%s] -> %v", count, destinationDevice, err)
	// }
	// fmt.Printf("\n")

	// ticker.Stop()
	return nil
}
