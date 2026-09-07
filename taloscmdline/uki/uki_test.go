package uki

import (
	"bytes"
	"debug/pe"
	"errors"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/tinkerbell/actions/taloscmdline/uki/ukitest"
)

func build(t *testing.T, sections []ukitest.Section, opts ukitest.Options) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), "Talos-v1.14.0.efi")
	if err := ukitest.Write(path, sections, opts); err != nil {
		t.Fatal(err)
	}

	return path
}

func nthSectionData(t *testing.T, f *pe.File, i int) []byte {
	t.Helper()

	s := f.Sections[i]
	data, err := io.ReadAll(io.LimitReader(s.Open(), int64(s.VirtualSize)))
	if err != nil {
		t.Fatal(err)
	}

	return data
}

func openPE(t *testing.T, path string) *pe.File {
	t.Helper()

	f, err := pe.Open(path)
	if err != nil {
		t.Fatalf("parsing %s: %v", path, err)
	}
	t.Cleanup(func() { f.Close() })

	return f
}

// assertLayout checks the invariants sd-stub and UEFI loaders rely on.
func assertLayout(t *testing.T, path string, cmdlines []string, original []ukitest.Section) {
	t.Helper()

	f := openPE(t, path)

	if len(f.Sections) != len(original) {
		t.Fatalf("expected %d sections, got %d", len(original), len(f.Sections))
	}

	header, ok := f.OptionalHeader.(*pe.OptionalHeader64)
	if !ok {
		t.Fatal("expected PE32+")
	}

	st, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}

	var prev *pe.Section
	cmdlineIndex := 0
	for i, s := range f.Sections {
		want := original[i]
		if s.Name != want.Name {
			t.Fatalf("section %d: expected %s, got %s", i, want.Name, s.Name)
		}

		wantData := want.Data
		if s.Name == ".cmdline" {
			wantData = []byte(cmdlines[cmdlineIndex])
			cmdlineIndex++
		}

		if got := nthSectionData(t, f, i); !bytes.Equal(got, wantData) {
			t.Fatalf("section %s: content changed", s.Name)
		}
		if int(s.VirtualSize) != len(wantData) {
			t.Fatalf("section %s: VirtualSize %d, want %d", s.Name, s.VirtualSize, len(wantData))
		}
		if s.Size%ukitest.FileAlignment != 0 || s.Size < s.VirtualSize {
			t.Fatalf("section %s: raw size %d not aligned or too small", s.Name, s.Size)
		}
		if s.VirtualAddress%ukitest.SectionAlignment != 0 {
			t.Fatalf("section %s: VirtualAddress %#x not aligned", s.Name, s.VirtualAddress)
		}
		if s.Offset%ukitest.FileAlignment != 0 {
			t.Fatalf("section %s: offset %#x not aligned", s.Name, s.Offset)
		}
		if prev != nil {
			if s.VirtualAddress < prev.VirtualAddress+prev.VirtualSize {
				t.Fatalf("section %s: VirtualAddress %#x overlaps %s", s.Name, s.VirtualAddress, prev.Name)
			}
			if s.Offset != prev.Offset+prev.Size {
				t.Fatalf("section %s: offset %#x not contiguous after %s", s.Name, s.Offset, prev.Name)
			}
		}

		prev = s
	}

	last := f.Sections[len(f.Sections)-1]
	wantImage := (uint64(last.VirtualAddress) + uint64(last.VirtualSize) + ukitest.SectionAlignment - 1) &^ (ukitest.SectionAlignment - 1)
	if uint64(header.SizeOfImage) != wantImage {
		t.Fatalf("SizeOfImage %#x, want %#x", header.SizeOfImage, wantImage)
	}
	if int64(last.Offset+last.Size) != st.Size() {
		t.Fatalf("file size %d, want %d (no trailing data)", st.Size(), last.Offset+last.Size)
	}
	if header.CheckSum != 0 {
		t.Fatalf("expected CheckSum to be zeroed, got %#x", header.CheckSum)
	}
	if sec := header.DataDirectory[pe.IMAGE_DIRECTORY_ENTRY_SECURITY]; sec.VirtualAddress != 0 || sec.Size != 0 {
		t.Fatalf("expected security directory to be empty, got %+v", sec)
	}
}

func TestInspectReadsCmdline(t *testing.T) {
	path := build(t, ukitest.TalosLike("talos.platform=metal console=ttyS0"), ukitest.Options{})

	info, err := Inspect(path)
	if err != nil {
		t.Fatalf("Inspect() error: %v", err)
	}
	want := []string{"talos.platform=metal console=ttyS0", "talos.platform=metal console=ttyS0" + ukitest.ResetSuffix}
	if len(info.Cmdlines) != 2 || info.Cmdlines[0] != want[0] || info.Cmdlines[1] != want[1] {
		t.Fatalf("unexpected cmdlines %q, want %q", info.Cmdlines, want)
	}
	if info.Signed {
		t.Fatal("expected an unsigned file")
	}
}

func TestInspectReportsSignature(t *testing.T) {
	path := build(t, ukitest.TalosLike("a=1"), ukitest.Options{Signed: true})

	info, err := Inspect(path)
	if err != nil {
		t.Fatalf("Inspect() error: %v", err)
	}
	if !info.Signed {
		t.Fatal("expected Signed to be reported")
	}
}

func TestRewriteGrowsCmdlineAcrossAlignment(t *testing.T) {
	original := ukitest.TalosLike(strings.Repeat("a", 100))
	path := build(t, original, ukitest.Options{})
	before := stubPrefix(t, path)

	newCmdlines := []string{strings.Repeat("b", 100) + " talos.config=" + strings.Repeat("c", 500), "reset=1 " + strings.Repeat("d", 700)}
	if err := Rewrite(path, newCmdlines, Options{}); err != nil {
		t.Fatalf("Rewrite() error: %v", err)
	}

	assertLayout(t, path, newCmdlines, original)

	if !bytes.Equal(stubPrefix(t, path), before) {
		t.Fatal("expected sections before .cmdline to be byte-identical")
	}
}

func TestRewriteShrinksCmdline(t *testing.T) {
	original := ukitest.TalosLike(strings.Repeat("a", 1500))
	path := build(t, original, ukitest.Options{})

	if err := Rewrite(path, []string{"short=1", "short=2"}, Options{}); err != nil {
		t.Fatalf("Rewrite() error: %v", err)
	}

	assertLayout(t, path, []string{"short=1", "short=2"}, original)
}

func TestRewriteSameRawSizeKeepsLaterOffsets(t *testing.T) {
	original := ukitest.TalosLike("a=1 b=2")
	path := build(t, original, ukitest.Options{})
	before := openPE(t, path)
	beforeInitrd := sectionOffset(before, ".initrd")

	if err := Rewrite(path, []string{"a=1 b=2 c=3", "a=1 b=2" + ukitest.ResetSuffix}, Options{}); err != nil {
		t.Fatalf("Rewrite() error: %v", err)
	}

	assertLayout(t, path, []string{"a=1 b=2 c=3", "a=1 b=2" + ukitest.ResetSuffix}, original)

	if got := sectionOffset(openPE(t, path), ".initrd"); got != beforeInitrd {
		t.Fatalf(".initrd moved from %#x to %#x although the raw size did not change", beforeInitrd, got)
	}
}

func TestRewriteRefusesSignedWithoutStrip(t *testing.T) {
	path := build(t, ukitest.TalosLike("a=1"), ukitest.Options{Signed: true})
	before, _ := os.ReadFile(path)

	err := Rewrite(path, []string{"a=2", "a=2" + ukitest.ResetSuffix}, Options{})
	if !errors.Is(err, ErrSigned) {
		t.Fatalf("expected ErrSigned, got %v", err)
	}

	after, _ := os.ReadFile(path)
	if !bytes.Equal(before, after) {
		t.Fatal("expected the file to be untouched")
	}
	assertNoTempFiles(t, filepath.Dir(path))
}

func TestRewriteStripsSignatureWhenAllowed(t *testing.T) {
	original := ukitest.TalosLike("a=1")
	path := build(t, original, ukitest.Options{Signed: true})

	if err := Rewrite(path, []string{"a=2", "a=2" + ukitest.ResetSuffix}, Options{StripSignature: true}); err != nil {
		t.Fatalf("Rewrite() error: %v", err)
	}

	assertLayout(t, path, []string{"a=2", "a=2" + ukitest.ResetSuffix}, original)

	info, err := Inspect(path)
	if err != nil || info.Signed {
		t.Fatalf("expected an unsigned file after stripping, signed=%v err=%v", info.Signed, err)
	}
}

func TestRewriteRefusesCodeAfterCmdline(t *testing.T) {
	sections := []ukitest.Section{
		{Name: ".osrel", Data: []byte("x"), Characteristics: ukitest.DataFlags},
		{Name: ".cmdline", Data: []byte("a=1"), Characteristics: ukitest.DataFlags},
		{Name: ".text", Data: ukitest.Pattern(1, 100), Characteristics: ukitest.CodeFlags},
	}
	path := build(t, sections, ukitest.Options{})

	if err := Rewrite(path, []string{"a=2"}, Options{}); err == nil {
		t.Fatal("expected an error when code follows .cmdline")
	}
}

func TestRewriteRefusesMissingCmdline(t *testing.T) {
	sections := []ukitest.Section{
		{Name: ".text", Data: ukitest.Pattern(1, 100), Characteristics: ukitest.CodeFlags},
		{Name: ".osrel", Data: []byte("x"), Characteristics: ukitest.DataFlags},
	}
	path := build(t, sections, ukitest.Options{})

	if err := Rewrite(path, []string{"a=2"}, Options{}); err == nil {
		t.Fatal("expected an error when .cmdline is missing")
	}
	if _, err := Inspect(path); err == nil {
		t.Fatal("expected Inspect to fail without .cmdline")
	}
}

func TestRewriteLeavesNoTempFiles(t *testing.T) {
	path := build(t, ukitest.TalosLike("a=1"), ukitest.Options{})

	if err := Rewrite(path, []string{"a=2", "a=2" + ukitest.ResetSuffix}, Options{}); err != nil {
		t.Fatalf("Rewrite() error: %v", err)
	}

	assertNoTempFiles(t, filepath.Dir(path))
}

func TestRewriteSingleCmdlineWithoutProfiles(t *testing.T) {
	original := ukitest.SingleCmdline("a=1")
	path := build(t, original, ukitest.Options{})

	info, err := Inspect(path)
	if err != nil || len(info.Cmdlines) != 1 || info.Cmdlines[0] != "a=1" {
		t.Fatalf("unexpected inspect result %+v err=%v", info, err)
	}

	if err := Rewrite(path, []string{"a=1 " + strings.Repeat("x", 900)}, Options{}); err != nil {
		t.Fatalf("Rewrite() error: %v", err)
	}

	assertLayout(t, path, []string{"a=1 " + strings.Repeat("x", 900)}, original)
}

func TestRewriteRejectsWrongCmdlineCount(t *testing.T) {
	path := build(t, ukitest.TalosLike("a=1"), ukitest.Options{})

	if err := Rewrite(path, []string{"a=2"}, Options{}); err == nil {
		t.Fatal("expected an error when the number of command lines does not match the sections")
	}
}

func stubPrefix(t *testing.T, path string) []byte {
	t.Helper()

	f := openPE(t, path)
	for _, s := range f.Sections {
		if s.Name == ".cmdline" {
			raw, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			// Skip the headers: they legitimately change (sizes, offsets).
			return raw[f.Sections[0].Offset:s.Offset]
		}
	}

	t.Fatal(".cmdline not found")

	return nil
}

func sectionOffset(f *pe.File, name string) uint32 {
	for _, s := range f.Sections {
		if s.Name == name {
			return s.Offset
		}
	}

	return 0
}

func assertNoTempFiles(t *testing.T, dir string) {
	t.Helper()

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if !strings.HasSuffix(e.Name(), ".efi") {
			t.Fatalf("unexpected file left behind: %s", e.Name())
		}
	}
}

func TestInspectReportsCmdlineLocations(t *testing.T) {
	path := build(t, ukitest.TalosLike("a=1"), ukitest.Options{})

	info, err := Inspect(path)
	if err != nil {
		t.Fatalf("Inspect() error: %v", err)
	}

	f := openPE(t, path)

	var want []Location
	for i, s := range f.Sections {
		if s.Name == SectionName {
			want = append(want, Location{Section: i, Offset: int64(s.Offset), Size: int64(s.VirtualSize)})
		}
	}

	if len(want) != 2 || !reflect.DeepEqual(info.Locations, want) {
		t.Fatalf("Locations = %+v, want %+v", info.Locations, want)
	}
}
