// Package uki reads and rewrites the kernel command line stored in the
// .cmdline sections of a unified kernel image.
//
// Talos assembles a UKI by appending data-only sections (.osrel, .cmdline,
// .uname, .splash, .linux, .initrd, then .profile sections with their own
// .cmdline) to the systemd stub, laying them out sequentially with
// SectionAlignment for virtual addresses and FileAlignment for raw sizes.
// Rewrite keeps everything before the first .cmdline byte-identical and
// re-lays out that section and everything after it with the same rules, so
// the result matches what Talos' own assembler would produce.
package uki

import (
	"bytes"
	"debug/pe"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// SectionName is the PE section holding a kernel command line.
const SectionName = ".cmdline"

// ErrSigned is returned when a UKI carries an Authenticode signature and
// stripping it was not allowed.
var ErrSigned = errors.New("UKI carries an Authenticode signature")

// Info describes a UKI.
type Info struct {
	Path string
	// Cmdlines holds every .cmdline section in file order: the default boot
	// first, then one per boot profile that overrides it.
	Cmdlines []string
	// Locations describes where each entry of Cmdlines is stored, in the
	// same order.
	Locations []Location
	Signed    bool
}

// Location is where a .cmdline section lives inside the UKI file.
type Location struct {
	// Section is the index in the PE section table.
	Section int
	// Offset is the byte offset of the section data in the file.
	Offset int64
	// Size is the length of the command line in bytes.
	Size int64
}

// Options control Rewrite.
type Options struct {
	// StripSignature allows rewriting a signed UKI; the signature is removed
	// and the file will no longer boot with Secure Boot enabled.
	StripSignature bool
}

const (
	dosHeaderSize        = 0x40
	peSignatureSize      = 4
	fileHeaderSize       = 20
	sectionHeaderSize    = 40
	optionalHeaderMagic  = 0x20b
	unsafeCharacteristic = pe.IMAGE_SCN_CNT_CODE | pe.IMAGE_SCN_MEM_EXECUTE | pe.IMAGE_SCN_MEM_WRITE

	// Offsets inside the PE32+ optional header.
	offSizeOfCode            = 4
	offSizeOfInitializedData = 8
	offSizeOfImage           = 56
	offCheckSum              = 64
	offDataDirectory         = 112

	// Offsets inside a section header.
	offVirtualSize      = 8
	offVirtualAddress   = 12
	offSizeOfRawData    = 16
	offPointerToRawData = 20

	tmpSuffix = ".tmp"
)

// layout is the parsed header structure of a UKI.
type layout struct {
	optional     pe.OptionalHeader64
	sections     []pe.SectionHeader
	optionalOff  uint32
	sectionTable uint32
	// cmdlines are the indexes of the .cmdline sections, ascending.
	cmdlines []int
}

func (l *layout) signed() bool {
	dir := l.optional.DataDirectory[pe.IMAGE_DIRECTORY_ENTRY_SECURITY]

	return dir.VirtualAddress != 0 && dir.Size != 0
}

func (l *layout) first() int {
	return l.cmdlines[0]
}

func align(v, a uint64) uint64 {
	return (v + a - 1) &^ (a - 1)
}

func parse(r io.ReaderAt) (*layout, error) {
	peFile, err := pe.NewFile(r)
	if err != nil {
		return nil, fmt.Errorf("parsing PE headers: %w", err)
	}
	defer peFile.Close()

	optional, ok := peFile.OptionalHeader.(*pe.OptionalHeader64)
	if !ok || optional.Magic != optionalHeaderMagic {
		return nil, errors.New("only PE32+ images are supported")
	}

	var dos [dosHeaderSize]byte
	if _, err := r.ReadAt(dos[:], 0); err != nil {
		return nil, fmt.Errorf("reading DOS header: %w", err)
	}

	peOffset := binary.LittleEndian.Uint32(dos[0x3c:])

	l := &layout{
		optional:    *optional,
		optionalOff: peOffset + peSignatureSize + fileHeaderSize,
	}
	l.sectionTable = l.optionalOff + uint32(peFile.SizeOfOptionalHeader)

	for i, s := range peFile.Sections {
		l.sections = append(l.sections, s.SectionHeader)

		if s.Name == SectionName {
			l.cmdlines = append(l.cmdlines, i)
		}
	}

	if len(l.cmdlines) == 0 {
		return nil, fmt.Errorf("no %s section found", SectionName)
	}

	return l, l.validate()
}

// validate checks the assumptions the rewrite relies on: sections are laid
// out in table order, and everything from the first .cmdline on is plain
// data that can be moved.
func (l *layout) validate() error {
	first := l.sections[l.first()]

	if uint64(l.sectionTable)+uint64(sectionHeaderSize)*uint64(len(l.sections)) > uint64(first.Offset) {
		return errors.New("section table overlaps the first .cmdline section")
	}

	for i, s := range l.sections {
		switch {
		case i < l.first():
			if uint64(s.Offset)+uint64(s.Size) > uint64(first.Offset) {
				return fmt.Errorf("section %s is not stored before %s", s.Name, SectionName)
			}
		case i > l.first():
			prev := l.sections[i-1]

			if s.Offset < prev.Offset+prev.Size {
				return fmt.Errorf("section %s is not stored after %s in table order", s.Name, prev.Name)
			}

			if s.VirtualAddress < prev.VirtualAddress+prev.VirtualSize {
				return fmt.Errorf("section %s virtual address overlaps %s", s.Name, prev.Name)
			}

			fallthrough
		default:
			if s.Characteristics&unsafeCharacteristic != 0 || s.Name == ".reloc" {
				return fmt.Errorf("section %s after %s is not plain data; refusing to relocate it", s.Name, SectionName)
			}
		}
	}

	return nil
}

func readCmdline(r io.ReaderAt, s pe.SectionHeader) (string, error) {
	n := s.VirtualSize
	if s.Size < n {
		n = s.Size
	}

	buf := make([]byte, n)
	if _, err := r.ReadAt(buf, int64(s.Offset)); err != nil {
		return "", fmt.Errorf("reading %s: %w", SectionName, err)
	}

	return string(bytes.TrimSpace(bytes.TrimRight(buf, "\x00"))), nil
}

// Inspect returns the command lines of the UKI at path and whether it is signed.
func Inspect(path string) (Info, error) {
	f, err := os.Open(path)
	if err != nil {
		return Info{}, err
	}
	defer f.Close()

	l, err := parse(f)
	if err != nil {
		return Info{}, fmt.Errorf("%s: %w", path, err)
	}

	info := Info{Path: path, Signed: l.signed()}

	for _, i := range l.cmdlines {
		cmdline, err := readCmdline(f, l.sections[i])
		if err != nil {
			return Info{}, fmt.Errorf("%s: %w", path, err)
		}

		info.Cmdlines = append(info.Cmdlines, cmdline)
		info.Locations = append(info.Locations, Location{
			Section: i,
			Offset:  int64(l.sections[i].Offset),
			Size:    int64(l.sections[i].VirtualSize),
		})
	}

	return info, nil
}

// Rewrite replaces the command lines of the UKI at path, one per .cmdline
// section in file order. The new file is written next to the original and
// renamed over it.
func Rewrite(path string, cmdlines []string, opts Options) error {
	in, err := os.Open(path)
	if err != nil {
		return err
	}
	defer in.Close()

	l, err := parse(in)
	if err != nil {
		return fmt.Errorf("%s: %w", path, err)
	}

	if len(cmdlines) != len(l.cmdlines) {
		return fmt.Errorf("%s: %d command lines given for %d %s sections", path, len(cmdlines), len(l.cmdlines), SectionName)
	}

	if l.signed() && !opts.StripSignature {
		return fmt.Errorf("%s: %w", path, ErrSigned)
	}

	tmp := path + tmpSuffix

	out, err := os.OpenFile(tmp, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o644)
	if err != nil {
		return fmt.Errorf("creating %s: %w", tmp, err)
	}

	if err := write(in, out, l, cmdlines); err != nil {
		out.Close()
		os.Remove(tmp)

		return fmt.Errorf("%s: %w", path, err)
	}

	if err := out.Sync(); err != nil {
		out.Close()
		os.Remove(tmp)

		return fmt.Errorf("syncing %s: %w", tmp, err)
	}

	if err := out.Close(); err != nil {
		os.Remove(tmp)

		return fmt.Errorf("closing %s: %w", tmp, err)
	}

	if err := os.Rename(tmp, path); err != nil {
		os.Remove(tmp)

		return fmt.Errorf("replacing %s: %w", path, err)
	}

	syncDir(filepath.Dir(path))

	return nil
}

// write produces the rewritten image: the original bytes up to the first
// .cmdline with patched headers, then every section from there on at its new
// offset. Anything after the last section (an Authenticode certificate
// table) is dropped.
func write(in io.ReaderAt, out io.Writer, l *layout, cmdlines []string) error {
	fileAlignment := uint64(l.optional.FileAlignment)
	sectionAlignment := uint64(l.optional.SectionAlignment)

	if fileAlignment == 0 || sectionAlignment == 0 {
		return errors.New("invalid alignment in optional header")
	}

	updated := make([]pe.SectionHeader, len(l.sections))
	copy(updated, l.sections)

	newData := make(map[int][]byte, len(cmdlines))

	for n, i := range l.cmdlines {
		data := []byte(cmdlines[n])
		newData[i] = data
		updated[i].VirtualSize = uint32(len(data))
		updated[i].Size = uint32(align(uint64(len(data)), fileAlignment))
	}

	first := l.first()
	va := uint64(updated[first].VirtualAddress)
	offset := uint64(updated[first].Offset)

	for i := first; i < len(updated); i++ {
		updated[i].VirtualAddress = uint32(va)
		updated[i].Offset = uint32(offset)

		va = align(va+uint64(updated[i].VirtualSize), sectionAlignment)
		offset += uint64(updated[i].Size)
	}

	last := updated[len(updated)-1]
	sizeOfImage := align(uint64(last.VirtualAddress)+uint64(last.VirtualSize), sectionAlignment)

	var sizeOfCode, sizeOfData uint32

	for _, s := range updated {
		if s.Characteristics&pe.IMAGE_SCN_CNT_INITIALIZED_DATA != 0 {
			sizeOfData += s.Size
		} else {
			sizeOfCode += s.Size
		}
	}

	// Headers and the sections before the first .cmdline, patched in place.
	prefix := make([]byte, l.sections[first].Offset)
	if _, err := in.ReadAt(prefix, 0); err != nil {
		return fmt.Errorf("reading headers: %w", err)
	}

	putU32 := func(off uint32, v uint32) {
		binary.LittleEndian.PutUint32(prefix[off:off+4], v)
	}

	putU32(l.optionalOff+offSizeOfCode, sizeOfCode)
	putU32(l.optionalOff+offSizeOfInitializedData, sizeOfData)
	putU32(l.optionalOff+offSizeOfImage, uint32(sizeOfImage))
	putU32(l.optionalOff+offCheckSum, 0)

	securityDir := l.optionalOff + offDataDirectory + 8*pe.IMAGE_DIRECTORY_ENTRY_SECURITY
	putU32(securityDir, 0)
	putU32(securityDir+4, 0)

	for i := first; i < len(updated); i++ {
		entry := l.sectionTable + uint32(i)*sectionHeaderSize

		putU32(entry+offVirtualSize, updated[i].VirtualSize)
		putU32(entry+offVirtualAddress, updated[i].VirtualAddress)
		putU32(entry+offSizeOfRawData, updated[i].Size)
		putU32(entry+offPointerToRawData, updated[i].Offset)
	}

	if _, err := out.Write(prefix); err != nil {
		return fmt.Errorf("writing headers: %w", err)
	}

	for i := first; i < len(updated); i++ {
		if data, ok := newData[i]; ok {
			if _, err := out.Write(data); err != nil {
				return fmt.Errorf("writing %s: %w", SectionName, err)
			}

			if _, err := out.Write(make([]byte, int(updated[i].Size)-len(data))); err != nil {
				return fmt.Errorf("padding %s: %w", SectionName, err)
			}

			continue
		}

		orig := l.sections[i]

		if _, err := io.Copy(out, io.NewSectionReader(in, int64(orig.Offset), int64(orig.Size))); err != nil {
			return fmt.Errorf("copying section %s: %w", orig.Name, err)
		}
	}

	return nil
}

// syncDir flushes the directory entry after the rename; vfat may not support
// it, so failures are ignored.
func syncDir(dir string) {
	d, err := os.Open(dir)
	if err != nil {
		return
	}
	defer d.Close()

	_ = d.Sync()
}
