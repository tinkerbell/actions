package ubootenv

import (
	"encoding/binary"
	"fmt"
	"hash/crc32"
	"sort"
	"strings"
)

// DefaultEnvSize is the default total size of a U-Boot environment file (0x4000 = 16384 bytes).
const DefaultEnvSize = 0x4000

// crcSize is the size of the CRC32 header in bytes.
const crcSize = 4

// Env represents a parsed U-Boot environment.
type Env struct {
	Vars map[string]string
	Size int
}

// Parse reads a U-Boot environment from raw bytes.
// The expected format is:
//
// [4 bytes CRC32 little-endian][data...]
//
// Data consists of null-terminated "key=value" strings.
// The end of variables is marked by a double null byte.
func Parse(data []byte) (*Env, error) {
	if len(data) < crcSize+1 {
		return nil, fmt.Errorf("data too short: %d bytes", len(data))
	}

	storedCRC := binary.LittleEndian.Uint32(data[:crcSize])
	payload := data[crcSize:]

	computedCRC := crc32.ChecksumIEEE(payload)
	if storedCRC != computedCRC {
		return nil, fmt.Errorf("CRC mismatch: stored=0x%08x computed=0x%08x", storedCRC, computedCRC)
	}

	vars := make(map[string]string)
	pos := 0
	for pos < len(payload) {
		if payload[pos] == 0 {
			break // end of environment
		}

		// Find the null terminator for this entry.
		end := pos
		for end < len(payload) && payload[end] != 0 {
			end++
		}

		entry := string(payload[pos:end])
		k, v, ok := strings.Cut(entry, "=")
		if !ok {
			return nil, fmt.Errorf("malformed entry at offset %d: %q", crcSize+pos, entry)
		}
		vars[k] = v

		pos = end + 1 // skip null terminator
	}

	return &Env{
		Vars: vars,
		Size: len(data),
	}, nil
}

// Marshal serializes the environment back to the binary format.
// Keys are sorted for deterministic output.
func (e *Env) Marshal() ([]byte, error) {
	if e.Size < crcSize+2 {
		return nil, fmt.Errorf("env size too small: %d", e.Size)
	}

	buf := make([]byte, e.Size)
	dataSize := e.Size - crcSize

	// Build the payload: sorted key=value pairs separated by null bytes.
	keys := make([]string, 0, len(e.Vars))
	for k := range e.Vars {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	pos := 0
	for _, k := range keys {
		entry := k + "=" + e.Vars[k]
		needed := len(entry) + 1 // +1 for null terminator
		if pos+needed+1 > dataSize {
			return nil, fmt.Errorf("environment data exceeds available space (%d bytes)", dataSize)
		}
		copy(buf[crcSize+pos:], entry)
		pos += len(entry)
		buf[crcSize+pos] = 0 // null terminator
		pos++
	}

	// The double-null terminator is already present since the buffer is zero-initialized.

	// Compute and store CRC32 over the data portion.
	payload := buf[crcSize:]
	checksum := crc32.ChecksumIEEE(payload)
	binary.LittleEndian.PutUint32(buf[:crcSize], checksum)

	return buf, nil
}
