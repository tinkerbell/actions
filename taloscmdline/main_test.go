package main

import (
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"testing"

	"github.com/tinkerbell/actions/taloscmdline/uki"
	"github.com/tinkerbell/actions/taloscmdline/uki/ukitest"
)

func quietLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func newEFITree(t *testing.T) string {
	t.Helper()

	root := t.TempDir()
	for _, name := range []string{"EFI/Linux/Talos-v1.14.0.efi", "EFI/Linux/Talos-v1.14.0~1.efi"} {
		path := filepath.Join(root, name)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := ukitest.Write(path, ukitest.TalosLike("talos.platform=metal console=ttyS0"), ukitest.Options{}); err != nil {
			t.Fatal(err)
		}
	}
	bootloader := filepath.Join(root, "EFI/boot/BOOTX64.efi")
	if err := os.MkdirAll(filepath.Dir(bootloader), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(bootloader, []byte("not a uki"), 0o644); err != nil {
		t.Fatal(err)
	}

	return root
}

func TestEditUKIsUpdatesEveryMatchAndIsIdempotent(t *testing.T) {
	root := newEFITree(t)
	args := "talos.config=http://192.168.1.2:7080/2009-04-04/user-data"

	updated, unchanged, err := editUKIs(root, defaultUKIPattern, args, false, quietLogger())
	if err != nil {
		t.Fatalf("editUKIs() error: %v", err)
	}
	if updated != 2 || unchanged != 0 {
		t.Fatalf("expected 2 updated / 0 unchanged, got %d / %d", updated, unchanged)
	}

	for _, name := range []string{"EFI/Linux/Talos-v1.14.0.efi", "EFI/Linux/Talos-v1.14.0~1.efi"} {
		info, err := uki.Inspect(filepath.Join(root, name))
		if err != nil {
			t.Fatalf("Inspect(%s) error: %v", name, err)
		}
		want := []string{
			"talos.platform=metal console=ttyS0 " + args,
			"talos.platform=metal console=ttyS0" + ukitest.ResetSuffix + " " + args,
		}
		if len(info.Cmdlines) != 2 || info.Cmdlines[0] != want[0] || info.Cmdlines[1] != want[1] {
			t.Fatalf("%s: unexpected cmdlines %q, want %q", name, info.Cmdlines, want)
		}
	}

	if got, _ := os.ReadFile(filepath.Join(root, "EFI/boot/BOOTX64.efi")); string(got) != "not a uki" {
		t.Fatal("expected files outside the pattern to be untouched")
	}

	updated, unchanged, err = editUKIs(root, defaultUKIPattern, args, false, quietLogger())
	if err != nil {
		t.Fatalf("second editUKIs() error: %v", err)
	}
	if updated != 0 || unchanged != 2 {
		t.Fatalf("expected 0 updated / 2 unchanged on rerun, got %d / %d", updated, unchanged)
	}
}

func TestEditUKIsFailsWithoutMatches(t *testing.T) {
	root := t.TempDir()

	if _, _, err := editUKIs(root, defaultUKIPattern, "a=1", false, quietLogger()); err == nil {
		t.Fatal("expected an error when no UKI matches")
	}
}

func TestEditUKIsRefusesSignedUnlessStripping(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "EFI/Linux/Talos-v1.14.0.efi")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := ukitest.Write(path, ukitest.TalosLike("a=1"), ukitest.Options{Signed: true}); err != nil {
		t.Fatal(err)
	}

	if _, _, err := editUKIs(root, defaultUKIPattern, "a=2", false, quietLogger()); err == nil {
		t.Fatal("expected an error for a signed UKI")
	}

	updated, _, err := editUKIs(root, defaultUKIPattern, "a=2", true, quietLogger())
	if err != nil || updated != 1 {
		t.Fatalf("expected the signed UKI to be rewritten with stripping, updated=%d err=%v", updated, err)
	}
}

func TestRunRequiresInputs(t *testing.T) {
	t.Setenv("DEST_DISK", "")
	t.Setenv("KERNEL_ARGS", "a=1")
	if err := run(quietLogger()); err == nil {
		t.Fatal("expected an error without DEST_DISK")
	}

	t.Setenv("DEST_DISK", "/dev/null")
	t.Setenv("KERNEL_ARGS", "")
	if err := run(quietLogger()); err == nil {
		t.Fatal("expected an error without KERNEL_ARGS")
	}
}
