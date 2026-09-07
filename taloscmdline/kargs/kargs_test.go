package kargs

import "testing"

func TestMerge(t *testing.T) {
	tests := []struct {
		name     string
		existing string
		set      string
		want     string
	}{
		{"append new key", "console=ttyS0 init_on_alloc=1", "talos.config=http://x/y", "console=ttyS0 init_on_alloc=1 talos.config=http://x/y"},
		{"replace existing key in place", "a=1 talos.config=old b=2", "talos.config=new", "a=1 talos.config=new b=2"},
		{"replace duplicate keys keeps one", "talos.config=old1 x talos.config=old2", "talos.config=new", "talos.config=new x"},
		{"flag appended once", "quiet", "quiet splash", "quiet splash"},
		{"multiple args", "a=1", "b=2 c", "a=1 b=2 c"},
		{"whitespace normalised", "  a=1 \t b=2 ", " c=3 ", "a=1 b=2 c=3"},
		{"empty existing", "", "a=1", "a=1"},
		{"empty set keeps existing", "a=1 b", "", "a=1 b"},
		{"value containing equals and colons", "a=1", "talos.config=http://h:7080/2009-04-04/user-data?x=1", "a=1 talos.config=http://h:7080/2009-04-04/user-data?x=1"},
		{"idempotent", "a=1 talos.config=new", "talos.config=new", "a=1 talos.config=new"},
		{"untouched keys with same prefix survive", "talos.platform=metal", "talos.config=u", "talos.platform=metal talos.config=u"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Merge(tt.existing, tt.set); got != tt.want {
				t.Fatalf("Merge(%q, %q) = %q, want %q", tt.existing, tt.set, got, tt.want)
			}
		})
	}
}
