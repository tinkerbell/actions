package archive

import (
	"archive/tar"
	"compress/gzip"
	"io"
	"os"
	"path/filepath"

	digest "github.com/opencontainers/go-digest"
	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
)

func extractTarDirectory(root string, r io.Reader) error {
	hardLinks := make(map[string]string)
	tr := tar.NewReader(r)
	for {
		header, err := tr.Next()
		if err != nil {
			if err == io.EOF {
				return nil
			}
			return err
		}
		path := filepath.Join(root, header.Name)
		info := header.FileInfo()
		switch header.Typeflag {
		case tar.TypeDir:
			if err = os.MkdirAll(path, info.Mode()); err != nil {
				return err
			}
			continue

		case tar.TypeLink:
			// Store details of hard links, which we process finally
			linkPath := filepath.Join(root, header.Linkname)
			linkPath2 := filepath.Join(root, header.Name)
			hardLinks[linkPath2] = linkPath
			continue

		case tar.TypeSymlink:
			linkPath := filepath.Join(root, header.Name)
			if err := os.Symlink(header.Linkname, linkPath); err != nil {
				if os.IsExist(err) {
					continue
				}
				return err
			}
			continue

		case tar.TypeReg:
			// Ensure any missing directories are created
			if _, err := os.Stat(filepath.Dir(path)); os.IsNotExist(err) {
				os.MkdirAll(filepath.Dir(path), 0755)
			}
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

	// To create hard links the targets must exist, so we do this finally
	for k, v := range hardLinks {
		if err := os.Link(v, k); err != nil {
			return err
		}
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
	var verifier digest.Verifier
	if checksum != "" {
		if digest, err := digest.Parse(checksum); err == nil {
			verifier = digest.Verifier()
			r = io.TeeReader(r, verifier)
		}
	}
	if err := extractTarDirectory(root, r); err != nil {
		return err
	}
	if verifier != nil && !verifier.Verified() {
		return errors.New("content digest mismatch")
	}
	return nil
}
