package image

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"
)

// errStalled reports that the source stopped delivering bytes.
var errStalled = errors.New("transfer stalled")

// newHTTPClient returns a client tuned for large sequential downloads.
//
// The default client has no timeout at any layer, so a server that accepts the
// connection and then stalls hangs the whole action forever. That also defeats
// RETRY_ENABLED, because backoff only reconsiders between attempts.
func newHTTPClient() *http.Client {
	base, ok := http.DefaultTransport.(*http.Transport)
	if !ok {
		return &http.Client{}
	}

	t := base.Clone()
	t.ResponseHeaderTimeout = defaultHeaderTimeout
	// Transparent gzip would make ContentLength -1 and every byte offset
	// meaningless, and we decompress explicitly anyway.
	t.DisableCompression = true

	return &http.Client{Transport: t}
}

// openSource issues the GET and returns the response, which the caller closes.
func openSource(ctx context.Context, client *http.Client, sourceImage string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, sourceImage, nil)
	if err != nil {
		return nil, fmt.Errorf("building request for %s: %w", sourceImage, err)
	}
	// Belt and braces alongside DisableCompression: some proxies key off the
	// request header rather than the transport.
	req.Header.Set("Accept-Encoding", "identity")

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetching %s: %w", sourceImage, err)
	}

	if resp.StatusCode != http.StatusOK {
		_ = resp.Body.Close()
		if resp.StatusCode == http.StatusNotFound {
			return nil, fmt.Errorf("%s not found", sourceImage)
		}
		return nil, fmt.Errorf("fetching %s: %s", sourceImage, resp.Status)
	}

	return resp, nil
}

// startStallWatchdog cancels the transfer when the read counter has not
// advanced for timeout. A non-positive timeout disables it. The returned
// function stops the watchdog.
func startStallWatchdog(ctx context.Context, ctr *counters, timeout time.Duration, cancel context.CancelCauseFunc) func() {
	if timeout <= 0 {
		return func() {}
	}

	done := make(chan struct{})
	go func() {
		// Sample several times per timeout so the detection granularity is
		// finer than the timeout itself.
		tick := max(timeout/4, time.Second)
		ticker := time.NewTicker(tick)
		defer ticker.Stop()

		last := ctr.read.Load()
		idle := time.Duration(0)

		for {
			select {
			case <-done:
				return
			case <-ctx.Done():
				return
			case <-ticker.C:
				now := ctr.read.Load()
				if now != last {
					last, idle = now, 0
					continue
				}
				idle += tick
				if idle >= timeout {
					cancel(fmt.Errorf("%w: no data for %s", errStalled, timeout))
					return
				}
			}
		}
	}()

	return func() { close(done) }
}
