package image

import (
	"crypto/tls"
	"net/http"

	"oras.land/oras-go/v2/registry/remote/auth"
	"oras.land/oras-go/v2/registry/remote/retry"
)

// newRegistryClient builds the HTTP client used to talk to the registry.
//
// A token cache is always configured so registries that answer every request
// with a bearer challenge (ghcr.io does this even for public images) can be
// pulled anonymously. Credentials, when supplied, are bound to the registry
// host of the image reference.
func newRegistryClient(registryHost, username, password string, skipVerify bool) *auth.Client {
	transport, ok := http.DefaultTransport.(*http.Transport)
	if ok {
		transport = transport.Clone()
	} else {
		transport = &http.Transport{Proxy: http.ProxyFromEnvironment}
	}

	if skipVerify {
		transport.TLSClientConfig = &tls.Config{InsecureSkipVerify: true} //nolint:gosec // G402: opt-in through SKIP_VERIFY, defaults to verifying.
	}

	client := &auth.Client{
		Client: &http.Client{Transport: retry.NewTransport(transport)},
		Cache:  auth.NewCache(),
	}
	client.SetUserAgent("oci2disk")

	if username != "" && password != "" {
		client.Credential = auth.StaticCredential(registryHost, auth.Credential{Username: username, Password: password})
	}

	return client
}
