package storage

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"
)

func TestMetadataUnmarshalsRAID(t *testing.T) {
	body := []byte(`{
	  "metadata": {
	    "instance": {
	      "storage": {
	        "raid": [
	          {"name": "md0", "level": "1", "devices": ["/dev/sdb1", "/dev/sdc1"], "spare": ["/dev/sdd1"]}
	        ]
	      }
	    }
	  }
	}`)

	var w Wrapper
	if err := json.Unmarshal(body, &w); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	got := w.Metadata.Instance.Storage.RAID
	if len(got) != 1 {
		t.Fatalf("want 1 raid entry, got %d", len(got))
	}
	want := RAID{
		Name:    "md0",
		Level:   "1",
		Devices: []string{"/dev/sdb1", "/dev/sdc1"},
		Spare:   []string{"/dev/sdd1"},
	}
	if !reflect.DeepEqual(got[0], want) {
		t.Errorf("got %+v, want %+v", got[0], want)
	}
}

func TestBuildMdadmCreateArgs_basic(t *testing.T) {
	r := RAID{Name: "md0", Level: "1", Devices: []string{"/dev/sdb1", "/dev/sdc1"}}
	args := BuildMdadmCreateArgs(r)
	want := []string{
		"--create", "/dev/md0",
		"--metadata=1.2",
		"--level=1",
		"--raid-devices=2",
		"--run", "--force",
		"/dev/sdb1", "/dev/sdc1",
	}
	if !reflect.DeepEqual(args, want) {
		t.Errorf("got %v\nwant %v", args, want)
	}
}

func TestBuildMdadmCreateArgs_withSpare(t *testing.T) {
	r := RAID{
		Name:    "md1",
		Level:   "5",
		Devices: []string{"/dev/sdb1", "/dev/sdc1", "/dev/sdd1"},
		Spare:   []string{"/dev/sde1"},
	}
	args := BuildMdadmCreateArgs(r)
	joined := strings.Join(args, " ")
	for _, want := range []string{
		"--create /dev/md1",
		"--level=5",
		"--raid-devices=3",
		"--spare-devices=1",
		"/dev/sdb1 /dev/sdc1 /dev/sdd1",
		"/dev/sde1",
	} {
		if !strings.Contains(joined, want) {
			t.Errorf("missing %q in %q", want, joined)
		}
	}
}

func TestBuildMdadmCreateArgs_normalizesDevicePath(t *testing.T) {
	cases := []struct{ in, wantDev string }{
		{"md0", "/dev/md0"},
		{"/dev/md0", "/dev/md0"},
		{"/dev/md/root", "/dev/md/root"},
	}
	for _, tc := range cases {
		r := RAID{Name: tc.in, Level: "0", Devices: []string{"/dev/sdb1", "/dev/sdc1"}}
		args := BuildMdadmCreateArgs(r)
		if args[1] != tc.wantDev {
			t.Errorf("name %q: got device %q, want %q", tc.in, args[1], tc.wantDev)
		}
	}
}

func TestValidateRAID(t *testing.T) {
	cases := []struct {
		name    string
		r       RAID
		wantErr string
	}{
		{"empty name", RAID{Level: "1", Devices: []string{"/dev/sdb1", "/dev/sdc1"}}, "name"},
		{"empty level", RAID{Name: "md0", Devices: []string{"/dev/sdb1", "/dev/sdc1"}}, "level"},
		{"bad level", RAID{Name: "md0", Level: "7", Devices: []string{"/dev/sdb1", "/dev/sdc1"}}, "level"},
		{"no devices", RAID{Name: "md0", Level: "1"}, "device"},
		{"raid1 needs 2", RAID{Name: "md0", Level: "1", Devices: []string{"/dev/sdb1"}}, "device"},
		{"raid5 needs 3", RAID{Name: "md0", Level: "5", Devices: []string{"/dev/sdb1", "/dev/sdc1"}}, "device"},
		{"valid raid0", RAID{Name: "md0", Level: "0", Devices: []string{"/dev/sdb1", "/dev/sdc1"}}, ""},
		{"valid raid1", RAID{Name: "md0", Level: "1", Devices: []string{"/dev/sdb1", "/dev/sdc1"}}, ""},
		{"valid raid10", RAID{Name: "md0", Level: "10", Devices: []string{"/dev/sdb1", "/dev/sdc1", "/dev/sdd1", "/dev/sde1"}}, ""},
		{"valid named /dev/md/root", RAID{Name: "/dev/md/root", Level: "1", Devices: []string{"/dev/sda2", "/dev/sdb2"}}, ""},
		{"valid named /dev/md/data_01", RAID{Name: "/dev/md/data_01", Level: "1", Devices: []string{"/dev/sda2", "/dev/sdb2"}}, ""},
		{"invalid bare /dev/md", RAID{Name: "/dev/md", Level: "1", Devices: []string{"/dev/sda2", "/dev/sdb2"}}, "name"},
		{"invalid trailing slash", RAID{Name: "/dev/md/", Level: "1", Devices: []string{"/dev/sda2", "/dev/sdb2"}}, "name"},
		{"invalid random path", RAID{Name: "/dev/root", Level: "1", Devices: []string{"/dev/sda2", "/dev/sdb2"}}, "name"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := ValidateRAID(tc.r)
			if tc.wantErr == "" {
				if err != nil {
					t.Errorf("want nil, got %v", err)
				}
				return
			}
			if err == nil {
				t.Fatalf("want error containing %q, got nil", tc.wantErr)
			}
			if !strings.Contains(strings.ToLower(err.Error()), tc.wantErr) {
				t.Errorf("want error containing %q, got %v", tc.wantErr, err)
			}
		})
	}
}
