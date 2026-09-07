// Package ukitest builds small PE files shaped like a Talos UKI for tests: a
// stub-like code section and relocations followed by the data sections Talos
// appends. The layout rules mirror Talos' native assembler.
package ukitest

import (
	"bytes"
	"debug/pe"
	"encoding/binary"
	"fmt"
	"os"
)

const (
	SectionAlignment = 0x1000
	FileAlignment    = 0x200

	dosHeaderSize = 0x80
	firstVA       = 0x1000
)

// DataFlags are the characteristics Talos gives appended sections.
const DataFlags = pe.IMAGE_SCN_CNT_INITIALIZED_DATA | pe.IMAGE_SCN_MEM_READ

// CodeFlags are the characteristics of the stub's code section.
const CodeFlags = pe.IMAGE_SCN_CNT_CODE | pe.IMAGE_SCN_MEM_EXECUTE | pe.IMAGE_SCN_MEM_READ

// Section is one section of the generated file.
type Section struct {
	Name            string
	Data            []byte
	Characteristics uint32
}

// Options tweak the generated file.
type Options struct {
	// Signed appends a fake Authenticode certificate table and points the
	// security data directory at it.
	Signed bool
}

func align(v, a uint64) uint64 {
	return (v + a - 1) &^ (a - 1)
}

// Write generates a PE32+ file at path.
func Write(path string, sections []Section, opts Options) error {
	var buf bytes.Buffer

	fileHeader := pe.FileHeader{
		Machine:              pe.IMAGE_FILE_MACHINE_AMD64,
		NumberOfSections:     uint16(len(sections)),
		SizeOfOptionalHeader: uint16(binary.Size(pe.OptionalHeader64{})),
		Characteristics:      pe.IMAGE_FILE_EXECUTABLE_IMAGE | pe.IMAGE_FILE_LARGE_ADDRESS_AWARE,
	}

	headersSize := uint32(align(uint64(dosHeaderSize+4+binary.Size(fileHeader)+binary.Size(pe.OptionalHeader64{})+binary.Size(pe.SectionHeader32{})*len(sections)), FileAlignment))

	headers := make([]pe.SectionHeader32, len(sections))
	va := uint64(firstVA)
	offset := uint64(headersSize)

	var sizeOfCode, sizeOfData uint32

	for i, s := range sections {
		raw := uint32(align(uint64(len(s.Data)), FileAlignment))
		copy(headers[i].Name[:], s.Name)
		headers[i].VirtualSize = uint32(len(s.Data))
		headers[i].VirtualAddress = uint32(va)
		headers[i].SizeOfRawData = raw
		headers[i].PointerToRawData = uint32(offset)
		headers[i].Characteristics = s.Characteristics

		if s.Characteristics&pe.IMAGE_SCN_CNT_INITIALIZED_DATA != 0 {
			sizeOfData += raw
		} else {
			sizeOfCode += raw
		}

		va = align(va+uint64(len(s.Data)), SectionAlignment)
		offset += uint64(raw)
	}

	optional := pe.OptionalHeader64{
		Magic:                 0x20b,
		SizeOfCode:            sizeOfCode,
		SizeOfInitializedData: sizeOfData,
		AddressOfEntryPoint:   firstVA,
		BaseOfCode:            firstVA,
		ImageBase:             0x140000000,
		SectionAlignment:      SectionAlignment,
		FileAlignment:         FileAlignment,
		MajorSubsystemVersion: 6,
		SizeOfImage:           uint32(va),
		SizeOfHeaders:         headersSize,
		Subsystem:             pe.IMAGE_SUBSYSTEM_EFI_APPLICATION,
		SizeOfStackReserve:    0x100000,
		SizeOfStackCommit:     0x1000,
		SizeOfHeapReserve:     0x100000,
		SizeOfHeapCommit:      0x1000,
		NumberOfRvaAndSizes:   16,
	}

	signature := []byte("FAKE-AUTHENTICODE-CERTIFICATE-TABLE-PADDED-TO-SIXTY-FOUR-BYTES!!")
	if opts.Signed {
		optional.DataDirectory[pe.IMAGE_DIRECTORY_ENTRY_SECURITY] = pe.DataDirectory{VirtualAddress: uint32(offset), Size: uint32(len(signature))}
	}

	// DOS header with e_lfanew pointing at the PE signature.
	dos := make([]byte, dosHeaderSize)
	copy(dos, "MZ")
	binary.LittleEndian.PutUint32(dos[0x3c:], dosHeaderSize)
	buf.Write(dos)
	buf.WriteString("PE\x00\x00")

	if err := binary.Write(&buf, binary.LittleEndian, fileHeader); err != nil {
		return err
	}
	if err := binary.Write(&buf, binary.LittleEndian, optional); err != nil {
		return err
	}
	for _, h := range headers {
		if err := binary.Write(&buf, binary.LittleEndian, h); err != nil {
			return err
		}
	}
	buf.Write(make([]byte, int(headersSize)-buf.Len()))

	for i, s := range sections {
		buf.Write(s.Data)
		buf.Write(make([]byte, int(headers[i].SizeOfRawData)-len(s.Data)))
	}

	if opts.Signed {
		buf.Write(signature)
	}

	if err := os.WriteFile(path, buf.Bytes(), 0o600); err != nil {
		return fmt.Errorf("writing %s: %w", path, err)
	}

	return nil
}

// Pattern returns n bytes of deterministic, non-repeating-looking content.
func Pattern(seed byte, n int) []byte {
	out := make([]byte, n)
	for i := range out {
		out[i] = byte(int(seed) + i*7 + i/251)
	}

	return out
}

// ResetSuffix is what Talos appends to the command line of its "reset" boot profile.
const ResetSuffix = " talos.experimental.wipe=system:EPHEMERAL,STATE"

// TalosLike returns the section list of a Talos 1.14 style UKI with the given
// command line: the base sections followed by two .profile sections and the
// .cmdline of the reset profile.
func TalosLike(cmdline string) []Section {
	return append(SingleCmdline(cmdline),
		Section{Name: ".profile", Data: []byte("ID=boot\n"), Characteristics: DataFlags},
		Section{Name: ".profile", Data: []byte("ID=reset\nTITLE=Reset Talos installation\n"), Characteristics: DataFlags},
		Section{Name: ".cmdline", Data: []byte(cmdline + ResetSuffix), Characteristics: DataFlags},
	)
}

// SingleCmdline returns the section list of a UKI without boot profiles.
func SingleCmdline(cmdline string) []Section {
	return []Section{
		{Name: ".text", Data: Pattern(1, 300), Characteristics: CodeFlags},
		{Name: ".rodata", Data: Pattern(2, 100), Characteristics: DataFlags},
		{Name: ".reloc", Data: Pattern(3, 20), Characteristics: DataFlags | pe.IMAGE_SCN_MEM_DISCARDABLE},
		{Name: ".osrel", Data: []byte("NAME=Talos\nID=talos\n"), Characteristics: DataFlags},
		{Name: ".cmdline", Data: []byte(cmdline), Characteristics: DataFlags},
		{Name: ".uname", Data: []byte("6.12.0-talos"), Characteristics: DataFlags},
		{Name: ".linux", Data: Pattern(4, 5000), Characteristics: DataFlags},
		{Name: ".initrd", Data: Pattern(5, 9000), Characteristics: DataFlags},
	}
}
