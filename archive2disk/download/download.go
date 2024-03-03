package download

import (
	"fmt"
	"io"
	"net/http"
	"time"
)

type fromURLOpts struct {
	Timeout time.Duration
}

// FromOption is an option for From
type FromOption func(*fromURLOpts)

// From returns a ReadCloser that streams the contents served from url. url must be HTTP.
func From(url string, opts ...FromOption) (io.ReadCloser, error) {
	var cfg fromURLOpts
	for _, fn := range opts {
		fn(&cfg)
	}

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}

	client := &http.Client{Timeout: cfg.Timeout}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode > 300 {
		return nil, fmt.Errorf("couldn't retrieve archive: HTTP %s", resp.Status)
	}

	return resp.Body, nil
}

// WithTimeout configures From to timeout after t.
func WithTimeout(t time.Duration) FromOption {
	return func(o *fromURLOpts) {
		o.Timeout = t
	}
}
