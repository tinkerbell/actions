package main

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"strings"
	"testing"

	diskfs "github.com/diskfs/go-diskfs"
	"github.com/diskfs/go-diskfs/partition/gpt"

	"github.com/tinkerbell/actions/talosmeta/meta"
)

func newTalosLikeDisk(t *testing.T) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), "disk.img")
	d, err := diskfs.Create(path, 8*1024*1024, diskfs.SectorSize512)
	if err != nil {
		t.Fatal(err)
	}
	table := &gpt.Table{
		Partitions: []*gpt.Partition{
			{Index: 1, Start: 2048, End: 4095, Type: gpt.EFISystemPartition, Name: "EFI"},
			{Index: 2, Start: 4096, End: 6143, Type: gpt.LinuxFilesystem, Name: "META"},
		},
		LogicalSectorSize:  512,
		PhysicalSectorSize: 512,
		ProtectiveMBR:      true,
	}
	if err := d.Partition(table); err != nil {
		t.Fatal(err)
	}
	if err := d.Close(); err != nil {
		t.Fatal(err)
	}

	return path
}

func quietLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func TestRunWritesNetworkConfigPassthrough(t *testing.T) {
	disk := newTalosLikeDisk(t)
	doc := "hostnames:\n    - hostname: node1\n      layer: platform\n"

	t.Setenv("DEST_DISK", disk)
	t.Setenv("NETWORK_CONFIG", doc)
	t.Setenv("MIRROR_HOST", "")

	if err := run(quietLogger()); err != nil {
		t.Fatalf("run() error: %v", err)
	}

	got, ok, err := meta.ReadTag(disk, meta.MetalNetworkPlatformConfig)
	if err != nil || !ok {
		t.Fatalf("expected tag 0xa to be written, ok=%v err=%v", ok, err)
	}
	if string(got) != doc {
		t.Fatalf("unexpected document written:\n%s", got)
	}
}

func TestRunRejectsInvalidPassthrough(t *testing.T) {
	t.Setenv("DEST_DISK", newTalosLikeDisk(t))
	t.Setenv("NETWORK_CONFIG", "addresses: [")

	if err := run(quietLogger()); err == nil || !strings.Contains(err.Error(), "NETWORK_CONFIG") {
		t.Fatalf("expected a NETWORK_CONFIG error, got %v", err)
	}
}

func TestRunBuildsDocumentFromMetadataService(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/metadata" {
			http.NotFound(w, r)
			return
		}
		_, _ = w.Write([]byte(`{"interfaces":[{"dhcp":{"mac":"52:54:00:12:34:01","iface_name":"enp1s0","hostname":"node1",
			"ip":{"address":"10.0.80.10","netmask":"255.255.255.0","gateway":"10.0.80.1","family":4}}}],
			"metadata":{"instance":{"hostname":"node1.example.com"}}}`))
	}))
	defer srv.Close()

	u, err := url.Parse(srv.URL)
	if err != nil {
		t.Fatal(err)
	}

	disk := newTalosLikeDisk(t)
	t.Setenv("DEST_DISK", disk)
	t.Setenv("NETWORK_CONFIG", "")
	t.Setenv("MIRROR_HOST", u.Hostname())
	t.Setenv("METADATA_SERVICE_PORT", u.Port())

	if err := run(quietLogger()); err != nil {
		t.Fatalf("run() error: %v", err)
	}

	got, ok, err := meta.ReadTag(disk, meta.MetalNetworkPlatformConfig)
	if err != nil || !ok {
		t.Fatalf("expected tag 0xa to be written, ok=%v err=%v", ok, err)
	}
	for _, want := range []string{"address: 10.0.80.10/24", "linkName: enp1s0", "gateway: 10.0.80.1", "hostname: node1", "domainname: example.com"} {
		if !strings.Contains(string(got), want) {
			t.Fatalf("expected document to contain %q:\n%s", want, got)
		}
	}
}

func TestRunFailsWhenMetadataHasNoInterfaces(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"metadata":{"instance":{"storage":{"disks":[]}}}}`))
	}))
	defer srv.Close()

	u, err := url.Parse(srv.URL)
	if err != nil {
		t.Fatal(err)
	}

	t.Setenv("DEST_DISK", newTalosLikeDisk(t))
	t.Setenv("NETWORK_CONFIG", "")
	t.Setenv("MIRROR_HOST", u.Hostname())
	t.Setenv("METADATA_SERVICE_PORT", u.Port())

	err = run(quietLogger())
	if err == nil || !strings.Contains(err.Error(), "no interfaces") {
		t.Fatalf("expected a no-interfaces error, got %v", err)
	}
}

func TestRunRequiresDestDisk(t *testing.T) {
	t.Setenv("DEST_DISK", "")

	if err := run(quietLogger()); err == nil || !strings.Contains(err.Error(), "DEST_DISK") {
		t.Fatalf("expected a DEST_DISK error, got %v", err)
	}
}

func TestRunRejectsUnknownLinkNaming(t *testing.T) {
	t.Setenv("DEST_DISK", newTalosLikeDisk(t))
	t.Setenv("NETWORK_CONFIG", "")
	t.Setenv("MIRROR_HOST", "127.0.0.1")
	t.Setenv("LINK_NAMING", "random")

	if err := run(quietLogger()); err == nil || !strings.Contains(err.Error(), "LINK_NAMING") {
		t.Fatalf("expected a LINK_NAMING error, got %v", err)
	}
}

const hardwareSpecJSON = `{"interfaces":[{"dhcp":{"mac":"52:54:00:12:34:01","iface_name":"enp1s0","hostname":"node1",
"name_servers":["10.0.0.2"],"ip":{"address":"10.0.80.10","netmask":"255.255.255.0","gateway":"10.0.80.1","family":4}}}],
"metadata":{"instance":{"hostname":"node1.example.com"}}}`

func TestRunBuildsDocumentFromHardwareSpec(t *testing.T) {
	disk := newTalosLikeDisk(t)
	t.Setenv("DEST_DISK", disk)
	t.Setenv("NETWORK_CONFIG", "")
	t.Setenv("MIRROR_HOST", "")
	t.Setenv("HARDWARE_SPEC", hardwareSpecJSON)

	if err := run(quietLogger()); err != nil {
		t.Fatalf("run() error: %v", err)
	}

	got, ok, err := meta.ReadTag(disk, meta.MetalNetworkPlatformConfig)
	if err != nil || !ok {
		t.Fatalf("expected tag 0xa to be written, ok=%v err=%v", ok, err)
	}
	for _, want := range []string{"address: 10.0.80.10/24", "linkName: enp1s0", "gateway: 10.0.80.1", "hostname: node1", "domainname: example.com", "- 10.0.0.2"} {
		if !strings.Contains(string(got), want) {
			t.Fatalf("expected document to contain %q:\n%s", want, got)
		}
	}
}

func TestRunHardwareSpecTakesPrecedenceOverMetadataService(t *testing.T) {
	disk := newTalosLikeDisk(t)
	t.Setenv("DEST_DISK", disk)
	t.Setenv("NETWORK_CONFIG", "")
	t.Setenv("MIRROR_HOST", "192.0.2.1") // unreachable on purpose
	t.Setenv("METADATA_SERVICE_PORT", "9")
	t.Setenv("HARDWARE_SPEC", hardwareSpecJSON)

	if err := run(quietLogger()); err != nil {
		t.Fatalf("run() error: %v", err)
	}
}

func TestRunRejectsInvalidHardwareSpec(t *testing.T) {
	t.Setenv("DEST_DISK", newTalosLikeDisk(t))
	t.Setenv("NETWORK_CONFIG", "")
	t.Setenv("HARDWARE_SPEC", "{not json")

	if err := run(quietLogger()); err == nil || !strings.Contains(err.Error(), "HARDWARE_SPEC") {
		t.Fatalf("expected a HARDWARE_SPEC error, got %v", err)
	}
}

func TestRunRejectsHardwareSpecWithoutInterfaces(t *testing.T) {
	t.Setenv("DEST_DISK", newTalosLikeDisk(t))
	t.Setenv("NETWORK_CONFIG", "")
	t.Setenv("HARDWARE_SPEC", `{"metadata":{"instance":{"hostname":"x"}}}`)

	if err := run(quietLogger()); err == nil || !strings.Contains(err.Error(), "no interfaces") {
		t.Fatalf("expected a no-interfaces error, got %v", err)
	}
}
