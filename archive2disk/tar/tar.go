package tar

// This package handles the pulling and management of images

import (
	"archive/tar"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	log "github.com/sirupsen/logrus"
)

func Extract(r io.Reader, root string) error {
	hardLinks := make(map[string]string)

	tr := tar.NewReader(r)
	for {
		header, err := tr.Next()
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			return err
		}

		path, err := sanitizePath(root, header.Name)
		if err != nil {
			return err
		}
		info := header.FileInfo()

		switch header.Typeflag {
		case tar.TypeDir:
			if err = os.MkdirAll(path, info.Mode()); err != nil {
				return err
			}

		case tar.TypeLink:
			// Store details of hard links as targets must exist before we can create them.
			dst, err := sanitizePath(root, header.Linkname)
			if err != nil {
				return err
			}

			hardLinks[path] = dst

		case tar.TypeSymlink:
			linkPath, err := sanitizePath(root, header.Name)
			if err != nil {
				return err
			}

			if err := os.Symlink(header.Linkname, linkPath); err != nil {
				if os.IsExist(err) {
					continue
				}
				return err
			}

		case tar.TypeReg:
			// Ensure any missing directories are created
			_, err := os.Stat(filepath.Dir(path))

			switch {
			case os.IsNotExist(err):
				if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
					return err
				}
			case err != nil:
				return err
			}

			file, err := os.OpenFile(path, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, info.Mode())
			if err != nil {
				return err
			}

			// Avoid malicious payloads consuming large uncontrolled amounts of RAM by chunking.
			for {
				_, err := io.CopyN(file, tr, 1024)
				if err != nil {
					// We're going to exit the loop no matter what so close the descriptor.
					file.Close()

					if errors.Is(err, io.EOF) {
						break
					}

					return err
				}
			}

		default:
			return fmt.Errorf("incompatible file type in tarball: %d", header.Typeflag)
		}
	}

	for k, v := range hardLinks {
		log.Infof("processing hard link [%s->%s]", v, k)
		if err := os.Link(v, k); err != nil {
			return err
		}
	}

	return nil
}

// sanitizePath sanitizeds t rooted at d by ensuring the file will actually be extracted under d.
func sanitizePath(d, t string) (v string, err error) {
	v = filepath.Join(d, t)
	if strings.HasPrefix(v, filepath.Clean(d)) {
		return v, nil
	}

	return "", fmt.Errorf("content filepath is tainted: %s", t)
}
