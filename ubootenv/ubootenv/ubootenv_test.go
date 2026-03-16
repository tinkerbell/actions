package ubootenv

import (
	"encoding/binary"
	"hash/crc32"
	"testing"
)

func makeEnv(size int, vars map[string]string) []byte {
	buf := make([]byte, size)
	pos := crcSize
	for k, v := range vars {
		entry := k + "=" + v
		copy(buf[pos:], entry)
		pos += len(entry)
		buf[pos] = 0
		pos++
	}
	crc := crc32.ChecksumIEEE(buf[crcSize:])
	binary.LittleEndian.PutUint32(buf[:crcSize], crc)
	return buf
}

func TestParseAndMarshalRoundTrip(t *testing.T) {
	original := map[string]string{
		"bootcmd":   "run distro_bootcmd",
		"bootdelay": "3",
		"ethaddr":   "00:11:22:33:44:55",
	}
	data := makeEnv(DefaultEnvSize, original)
	env, err := Parse(data)
	if err != nil {
		t.Fatalf("Parse() error: %v", err)
	}
	if len(env.Vars) != len(original) {
		t.Fatalf("expected %d vars, got %d", len(original), len(env.Vars))
	}
	for k, want := range original {
		got, ok := env.Vars[k]
		if !ok {
			t.Errorf("missing key %q", k)
			continue
		}
		if got != want {
			t.Errorf("key %q: got %q, want %q", k, got, want)
		}
	}
	out, err := env.Marshal()
	if err != nil {
		t.Fatalf("Marshal() error: %v", err)
	}
	if len(out) != DefaultEnvSize {
		t.Fatalf("expected output size %d, got %d", DefaultEnvSize, len(out))
	}
	env2, err := Parse(out)
	if err != nil {
		t.Fatalf("Parse(round-trip) error: %v", err)
	}
	for k, want := range original {
		got := env2.Vars[k]
		if got != want {
			t.Errorf("round-trip key %q: got %q, want %q", k, got, want)
		}
	}
}

func TestParseInvalidCRC(t *testing.T) {
	data := makeEnv(DefaultEnvSize, map[string]string{"foo": "bar"})
	data[0] ^= 0xFF
	_, err := Parse(data)
	if err == nil {
		t.Fatal("expected CRC mismatch error")
	}
}

func TestParseTooShort(t *testing.T) {
	_, err := Parse([]byte{0, 0, 0})
	if err == nil {
		t.Fatal("expected error for short data")
	}
}

func TestParseEmptyEnvironment(t *testing.T) {
	data := makeEnv(DefaultEnvSize, map[string]string{})
	env, err := Parse(data)
	if err != nil {
		t.Fatalf("Parse() error: %v", err)
	}
	if len(env.Vars) != 0 {
		t.Fatalf("expected 0 vars, got %d", len(env.Vars))
	}
}

func TestSetAndDeleteVars(t *testing.T) {
	original := map[string]string{
		"bootcmd":   "bootm",
		"bootdelay": "5",
		"toremove":  "gone",
	}
	data := makeEnv(DefaultEnvSize, original)
	env, err := Parse(data)
	if err != nil {
		t.Fatalf("Parse() error: %v", err)
	}
	env.Vars["newvar"] = "hello"
	env.Vars["bootdelay"] = "1"
	delete(env.Vars, "toremove")
	out, err := env.Marshal()
	if err != nil {
		t.Fatalf("Marshal() error: %v", err)
	}
	env2, err := Parse(out)
	if err != nil {
		t.Fatalf("Parse(after modification) error: %v", err)
	}
	expected := map[string]string{
		"bootcmd":   "bootm",
		"bootdelay": "1",
		"newvar":    "hello",
	}
	if len(env2.Vars) != len(expected) {
		t.Fatalf("expected %d vars, got %d", len(expected), len(env2.Vars))
	}
	for k, want := range expected {
		if got := env2.Vars[k]; got != want {
			t.Errorf("key %q: got %q, want %q", k, got, want)
		}
	}
	if _, ok := env2.Vars["toremove"]; ok {
		t.Error("expected toremove to be deleted")
	}
}

func TestMarshalOverflow(t *testing.T) {
	env := &Env{
		Vars: make(map[string]string),
		Size: 20,
	}
	env.Vars["a_very_long_variable_name"] = "a_very_long_value_that_exceeds_capacity"
	_, err := env.Marshal()
	if err == nil {
		t.Fatal("expected overflow error")
	}
}
