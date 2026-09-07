package talosnet

import (
	"os"
	"path/filepath"
	"testing"
)

// fakeSysfs builds the subset of sysfs that link naming looks at.
type fakeSysfs struct {
	t    *testing.T
	root string
}

func newFakeSysfs(t *testing.T) *fakeSysfs {
	t.Helper()
	return &fakeSysfs{t: t, root: t.TempDir()}
}

// addLink registers a network interface. devicePath is relative to root, e.g.
// "devices/pci0000:00/0000:00:03.0/virtio0"; empty means no device link.
func (f *fakeSysfs) addLink(name, mac, devicePath string) {
	f.t.Helper()
	dir := filepath.Join(f.root, "class", "net", name)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		f.t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "address"), []byte(mac+"\n"), 0o644); err != nil {
		f.t.Fatal(err)
	}
	if devicePath == "" {
		return
	}
	abs := filepath.Join(f.root, devicePath)
	if err := os.MkdirAll(abs, 0o755); err != nil {
		f.t.Fatal(err)
	}
	rel, err := filepath.Rel(dir, abs)
	if err != nil {
		f.t.Fatal(err)
	}
	if err := os.Symlink(rel, filepath.Join(dir, "device")); err != nil {
		f.t.Fatal(err)
	}
}

func (f *fakeSysfs) writeAttr(devicePath, attr, value string) {
	f.t.Helper()
	if err := os.WriteFile(filepath.Join(f.root, devicePath, attr), []byte(value+"\n"), 0o644); err != nil {
		f.t.Fatal(err)
	}
}

func (f *fakeSysfs) addSlot(slot int, address string) {
	f.t.Helper()
	dir := filepath.Join(f.root, "bus", "pci", "slots", itoa(slot))
	if err := os.MkdirAll(dir, 0o755); err != nil {
		f.t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "address"), []byte(address+"\n"), 0o644); err != nil {
		f.t.Fatal(err)
	}
}

func itoa(i int) string {
	return string(rune('0' + i))
}

func TestSysfsNamerKernelName(t *testing.T) {
	fs := newFakeSysfs(t)
	fs.addLink("eth0", "52:54:00:12:34:01", "devices/pci0000:00/0000:00:03.0/virtio0")
	fs.addLink("eth1", "52:54:00:12:34:02", "devices/pci0000:00/0000:00:04.0/virtio1")

	namer := &SysfsNamer{Root: fs.root, Predictable: false}
	got, err := namer.Name("52:54:00:12:34:02")
	if err != nil {
		t.Fatalf("Name() error: %v", err)
	}
	if got != "eth1" {
		t.Fatalf("expected eth1, got %q", got)
	}
}

func TestSysfsNamerMatchesMACCaseInsensitively(t *testing.T) {
	fs := newFakeSysfs(t)
	fs.addLink("eth0", "52:54:00:ab:cd:ef", "")

	namer := &SysfsNamer{Root: fs.root, Predictable: false}
	got, err := namer.Name("52:54:00:AB:CD:EF")
	if err != nil {
		t.Fatalf("Name() error: %v", err)
	}
	if got != "eth0" {
		t.Fatalf("expected eth0, got %q", got)
	}
}

func TestSysfsNamerUnknownMAC(t *testing.T) {
	fs := newFakeSysfs(t)
	fs.addLink("eth0", "52:54:00:12:34:01", "")

	namer := &SysfsNamer{Root: fs.root}
	if _, err := namer.Name("52:54:00:ff:ff:ff"); err == nil {
		t.Fatal("expected an error for an unknown MAC")
	}
}

func TestSysfsNamerPredictable(t *testing.T) {
	tests := []struct {
		name       string
		devicePath string
		setup      func(fs *fakeSysfs)
		want       string
	}{
		{
			name:       "pci path for virtio child",
			devicePath: "devices/pci0000:00/0000:00:03.0/virtio0",
			want:       "enp0s3",
		},
		{
			name:       "pci path with function",
			devicePath: "devices/pci0000:00/0000:00:1c.0/0000:01:00.1",
			want:       "enp1s0f1",
		},
		{
			name:       "pci path with non-zero domain",
			devicePath: "devices/pci0001:00/0001:02:00.0",
			want:       "enP1p2s0",
		},
		{
			name:       "onboard acpi index wins",
			devicePath: "devices/pci0000:00/0000:00:1f.6",
			setup:      func(fs *fakeSysfs) { fs.writeAttr("devices/pci0000:00/0000:00:1f.6", "acpi_index", "1") },
			want:       "eno1",
		},
		{
			name:       "onboard index attribute",
			devicePath: "devices/pci0000:00/0000:00:19.0",
			setup:      func(fs *fakeSysfs) { fs.writeAttr("devices/pci0000:00/0000:00:19.0", "index", "2") },
			want:       "eno2",
		},
		{
			name:       "hotplug slot",
			devicePath: "devices/pci0000:00/0000:00:1c.0/0000:03:00.0",
			setup:      func(fs *fakeSysfs) { fs.addSlot(3, "0000:03:00") },
			want:       "ens3",
		},
		{
			name:       "hotplug slot on parent bridge",
			devicePath: "devices/pci0000:00/0000:00:1c.0/0000:03:00.0",
			setup:      func(fs *fakeSysfs) { fs.addSlot(5, "0000:00:1c") },
			want:       "ens5",
		},
		{
			name:       "non pci device keeps kernel name",
			devicePath: "devices/platform/soc/fe300000.ethernet",
			want:       "eth0",
		},
		{
			name:       "no device link keeps kernel name",
			devicePath: "",
			want:       "eth0",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fs := newFakeSysfs(t)
			fs.addLink("eth0", "52:54:00:12:34:01", tt.devicePath)
			if tt.setup != nil {
				tt.setup(fs)
			}

			namer := &SysfsNamer{Root: fs.root, Predictable: true}
			got, err := namer.Name("52:54:00:12:34:01")
			if err != nil {
				t.Fatalf("Name() error: %v", err)
			}
			if got != tt.want {
				t.Fatalf("expected %q, got %q", tt.want, got)
			}
		})
	}
}
