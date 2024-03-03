package checksum

import (
	"io"

	"github.com/opencontainers/go-digest"
)

// Verifier returns true if the underlying digest matches the checksum.
type Verifier func() bool

// WithDigestVerification verifies the contents read from r match the digests specified by
// checksum. checksum should be formatted as <algorithm>:<hash>. For example sha256:123...
// If checksum is empty, a noop verifier is returned.
func VerifyStream(r io.Reader, checksum string) (io.Reader, Verifier, error) {
	if checksum == "" {
		return r, func() bool { return true }, nil
	}

	dd, err := digest.Parse(checksum)
	if err != nil {
		return nil, nil, err
	}
	verifier := dd.Verifier()

	return io.TeeReader(r, verifier), verifier.Verified, nil
}
