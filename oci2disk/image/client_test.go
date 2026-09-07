package image

import (
	"context"
	"net/http"
	"testing"

	"oras.land/oras-go/v2/registry/remote/auth"
	"oras.land/oras-go/v2/registry/remote/retry"
)

func baseTransport(t *testing.T, client *auth.Client) *http.Transport {
	t.Helper()

	rt := client.Client.Transport
	if r, ok := rt.(*retry.Transport); ok {
		rt = r.Base
	}
	transport, ok := rt.(*http.Transport)
	if !ok {
		t.Fatalf("expected *http.Transport, got %T", rt)
	}

	return transport
}

func TestNewRegistryClientUsesStaticCredentials(t *testing.T) {
	client := newRegistryClient("192.168.0.173", "foo", "bar", false)

	if client.Credential == nil {
		t.Fatal("expected a credential function")
	}
	cred, err := client.Credential(context.Background(), "192.168.0.173")
	if err != nil {
		t.Fatalf("Credential() error: %v", err)
	}
	if cred.Username != "foo" || cred.Password != "bar" {
		t.Fatalf("expected foo/bar credentials, got %q/%q", cred.Username, cred.Password)
	}
}

func TestNewRegistryClientAnonymousWhenNoCredentials(t *testing.T) {
	client := newRegistryClient("ghcr.io", "", "", false)

	// oras treats a nil credential func as anonymous.
	if client.Credential != nil {
		cred, err := client.Credential(context.Background(), "ghcr.io")
		if err != nil {
			t.Fatalf("Credential() error: %v", err)
		}
		if cred.Username != "" || cred.Password != "" || cred.AccessToken != "" {
			t.Fatalf("expected empty credentials, got %+v", cred)
		}
	}
	if client.Cache == nil {
		t.Fatal("expected a token cache so bearer challenges are answered once")
	}
}

func TestNewRegistryClientSkipVerify(t *testing.T) {
	client := newRegistryClient("registry.local", "", "", true)

	transport := baseTransport(t, client)
	if transport.TLSClientConfig == nil || !transport.TLSClientConfig.InsecureSkipVerify {
		t.Fatal("expected InsecureSkipVerify to be set")
	}
}

func TestNewRegistryClientVerifiesByDefault(t *testing.T) {
	client := newRegistryClient("registry.local", "", "", false)

	transport := baseTransport(t, client)
	if transport.TLSClientConfig != nil && transport.TLSClientConfig.InsecureSkipVerify {
		t.Fatal("expected TLS verification to stay enabled")
	}
}
