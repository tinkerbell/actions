package archive

import (
	"archive/tar"
	"compress/gzip"
	"io"
	"os"
	"path/filepath"
	// See https://github.com/opencontainers/go-digest#usage
	_ "crypto/sha256"
	_ "crypto/sha512"
	"fmt"

	digest "github.com/opencontainers/go-digest"
	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
)

func extractTarDirectory(root string, checksum string, r io.Reader) error {
	var verifier digest.Verifier
	if checksum != "" {
		log.Infof("checksum validation during untar [%s]", checksum)
		digest, err := digest.Parse(checksum)
		if err != nil {
			return fmt.Errorf("failed to parse digest - %w", err)
		}
		verifier = digest.Verifier()
		r = io.TeeReader(r, verifier)
	}
	hardLinks := make(map[string]string)
	tr := tar.NewReader(r)
	for {
		header, err := tr.Next()
		if err == io.EOF { /*end of file reached */
			break
		} else if err != nil {
			return err
		}
		path := filepath.Join(root, header.Name)
		info := header.FileInfo()
		switch header.Typeflag {
		case tar.TypeDir:
			log.Infof("processing directory [%s]", path)
			if err = os.MkdirAll(path, info.Mode()); err != nil {
				return err
			}
			continue

		case tar.TypeLink:
			// Store details of hard links
			linkPath := filepath.Join(root, header.Linkname)
			linkPath2 := filepath.Join(root, header.Name)
			hardLinks[linkPath2] = linkPath
		case tar.TypeSymlink:
			linkPath := filepath.Join(root, header.Name)
			log.Infof("processing sym link [%s]", linkPath)
			if err := os.Symlink(header.Linkname, linkPath); err != nil {
				if os.IsExist(err) {
					continue
				}
				return err
			}
		case tar.TypeReg:
			// Ensure any missing directories are created
			if _, err := os.Stat(filepath.Dir(path)); os.IsNotExist(err) {
				os.MkdirAll(filepath.Dir(path), 0o755)
			}
			log.Infof("processing file [%s]", path)
			file, err := os.OpenFile(path, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, info.Mode())
			if os.IsExist(err) {
				continue
			}
			if err != nil {
				return err
			}
			_, err = io.Copy(file, tr)
			file.Close()
			if err != nil {
				return err
			}
		default:
			log.Printf("Warning: File type %d unhandled by untar function!\n", header.Typeflag)
		}
	}

	// To create hard links the targets must exist
	for k, v := range hardLinks {
		log.Infof("processing hard link [%s->%s]", v, k)
		if err := os.Link(v, k); err != nil {
			return err
		}
	}

	if verifier != nil && !verifier.Verified() {
		return errors.New("digest mismatch")
	}

	return nil
}

func extractTarGzip(root string, checksum string, g io.Reader) error {
	zr, err := gzip.NewReader(g)
	if err != nil {
		return err
	}
	defer zr.Close()
	var r io.Reader = zr
	return extractTarDirectory(root, checksum, r)
}
