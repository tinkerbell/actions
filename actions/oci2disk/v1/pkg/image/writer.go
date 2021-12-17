package image

import (
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"log"

	"github.com/deislabs/oras/pkg/content"
	"github.com/opencontainers/go-digest"

	ctrcontent "github.com/containerd/containerd/content"
)

// DiskImageStore -
type DiskImageStore struct {
	sourceImage string
	writer      io.Writer
	compressed  bool
}

// NewDiskImageStore -
func NewDiskImageStore(sourceImage string, compressed bool, w io.Writer) DiskImageStore {
	// we have to reprocess the opts to find the blocksize
	// var wOpts := content.DefaultWriterOpts()
	// for _, opt := range opts {
	// 	if err := opt(&wOpts); err != nil {
	// 		// TODO: we probably should handle errors here
	// 		continueå
	// 	}
	// }
	return DiskImageStore{sourceImage: sourceImage, writer: w, compressed: compressed}
}

// Writer get a writer
func (d DiskImageStore) Writer(ctx context.Context, opts ...ctrcontent.WriterOpt) (ctrcontent.Writer, error) {
	// the logic is straightforward:
	// - if there is a desc in the opts, and the mediatype is tar or tar+gzip, then pass the correct decompress writer
	// - else, pass the regular writer
	// var (
	// 	writer        ctrcontent.Writer
	// 	err           error
	// 	multiIngester MultiWriterIngester
	// 	ok            bool
	// )

	// // check to see if we are supposed to use a MultiWriterIngester
	// if d.multiWriterIngester {
	// 	multiIngester, ok = d.ingester.(MultiWriterIngester)
	// 	if !ok {
	// 		return nil, errors.New("configured to use multiwriter ingester, but ingester does not implement multiwriter")
	// 	}
	// }

	// // we have to reprocess the opts to find the desc
	// var wOpts ctrcontent.WriterOpts
	// for _, opt := range opts {
	// 	if err := opt(&wOpts); err != nil {
	// 		return nil, err
	// 	}
	// }
	// // figure out if compression and/or archive exists
	// desc := wOpts.Desc
	// // before we pass it down, we need to strip anything we are removing here
	// // and possibly update the digest, since the store indexes things by digest
	// hasGzip, hasTar, modifiedMediaType := checkCompression(desc.MediaType)
	// wOpts.Desc.MediaType = modifiedMediaType
	// opts = append(opts, ctrcontent.WithDescriptor(wOpts.Desc))
	// // determine if we pass it blocksize, only if positive
	// writerOpts := []WriterOpt{}
	// if d.blocksize > 0 {
	// 	writerOpts = append(writerOpts, WithBlocksize(d.blocksize))
	// }

	// writer, err = d.ingester.Writer(ctx, opts...)
	// if err != nil {
	// 	return nil, err
	// }

	// // do we need to wrap with an untar writer?
	// if hasTar {
	// 	// if not multiingester, get a regular writer
	// 	if multiIngester == nil {
	// 		writer = NewUntarWriter(writer, writerOpts...)
	// 	} else {
	// 		writers, err := multiIngester.Writers(ctx, opts...)
	// 		if err != nil {
	// 			return nil, err
	// 		}
	// 		writer = NewUntarWriterByName(writers, writerOpts...)
	// 	}
	// }
	// if hasGzip {
	// 	if writer == nil {
	// 		writer, err = d.ingester.Writer(ctx, opts...)
	// 		if err != nil {
	// 			return nil, err
	// 		}
	// 	}
	// 	writer = NewGunzipWriter(writer, writerOpts...)
	// }
	// return writer, nil
	var f func(r io.Reader, w io.Writer, done chan<- error)

	wOpts := content.DefaultWriterOpts()

	di := &DiskImage{
		writer:   d.writer,
		digester: digest.Canonical.Digester(),
		// we take the OutputHash, since the InputHash goes to the passthrough writer,
		// which then passes the processed output to us
		// hash: wOpts.OutputHash,
	}

	// we have to reprocess the opts to find the desc
	var wOpts1 ctrcontent.WriterOpts
	for _, opt := range opts {
		if err := opt(&wOpts1); err != nil {
			return nil, err
		}
	}
	// figure out if compression and/or archive exists
	desc := wOpts1.Desc
	fmt.Printf("%v\n", desc.Annotations["org.opencontainers.image.title"])
	if desc.Annotations["org.opencontainers.image.title"] == "" {
		return content.NewIoContentWriter(ioutil.Discard, content.WithOutputHash(desc.Digest)), nil
	}
	if !d.compressed {
		// Without compression send raw output
		f = func(r io.Reader, w io.Writer, done chan<- error) {
			var err error
			b := make([]byte, wOpts.Blocksize)
			_, err = io.CopyBuffer(w, r, b)
			done <- err
		}
	} else {
		f = func(r io.Reader, w io.Writer, done chan<- error) {
			var err error
			decompressReader, err := findDecompressor(d.sourceImage, r)
			if err != nil {
				log.Fatalf(err.Error())
				done <- err
			} else {
				b := make([]byte, wOpts.Blocksize)
				_, err = io.CopyBuffer(w, decompressReader, b)
				done <- err
			}
		}
	}
	writerOpts := []content.WriterOpt{}
	return content.NewPassthroughWriter(di, f, writerOpts...), nil
	// return nil, err
}

// DiskImage -
type DiskImage struct {
	writer   io.Writer
	digester digest.Digester
	size     int64
	hash     *digest.Digest
}

func (w *DiskImage) Write(p []byte) (n int, err error) {
	n, err = w.writer.Write(p)
	if err != nil {
		return 0, err
	}
	w.size += int64(n)
	if w.hash == nil {
		w.digester.Hash().Write(p[:n])
	}
	return
}

// Close -
func (w *DiskImage) Close() error {
	return nil
}

// Digest may return empty digest or panics until committed.
func (w *DiskImage) Digest() digest.Digest {
	return w.digester.Digest()
}

// Commit commits the blob (but no roll-back is guaranteed on an error).
// size and expected can be zero-value when unknown.
// Commit always closes the writer, even on error.
// ErrAlreadyExists aborts the writer.
func (w *DiskImage) Commit(ctx context.Context, size int64, expected digest.Digest, opts ...ctrcontent.Opt) error {
	return nil
}

// Status returns the current state of write
func (w *DiskImage) Status() (ctrcontent.Status, error) {
	return ctrcontent.Status{}, nil
}

// Truncate updates the size of the target blob
func (w *DiskImage) Truncate(size int64) error {
	return nil
}
