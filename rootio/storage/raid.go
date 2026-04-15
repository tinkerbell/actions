package storage

import (
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"

	log "github.com/sirupsen/logrus"
)

// RAID defines a software RAID array to assemble with mdadm.
type RAID struct {
	Name    string   `json:"name"`
	Level   string   `json:"level"`
	Devices []string `json:"devices"`
	Spare   []string `json:"spare,omitempty"`
}

// Accept both the numeric mdadm form (md0, /dev/md0) and the udev-style
// named form (/dev/md/<name>). The named form is what mdadm produces when
// --name is used and is common in Equinix/Packet CPR metadata.
var raidNameRegexp = regexp.MustCompile(`^(/dev/)?md([0-9]+|/[A-Za-z][A-Za-z0-9_.-]*)$`)

// validLevels maps supported RAID levels to the minimum number of data devices.
var validLevels = map[string]int{
	"0":      2,
	"1":      2,
	"4":      3,
	"5":      3,
	"6":      4,
	"10":     4,
	"linear": 2,
}

// ValidateRAID checks a RAID entry against basic requirements.
func ValidateRAID(r RAID) error {
	if r.Name == "" {
		return fmt.Errorf("raid: name is required")
	}
	if !raidNameRegexp.MatchString(r.Name) {
		return fmt.Errorf("raid: invalid name %q (expected mdX or /dev/mdX)", r.Name)
	}
	if r.Level == "" {
		return fmt.Errorf("raid: level is required")
	}
	minDevs, ok := validLevels[r.Level]
	if !ok {
		return fmt.Errorf("raid: unsupported level %q", r.Level)
	}
	if len(r.Devices) < minDevs {
		return fmt.Errorf("raid: level %s requires at least %d devices, got %d", r.Level, minDevs, len(r.Devices))
	}
	return nil
}

// normalizeRAIDDevice returns the full /dev/ path for a RAID array name.
func normalizeRAIDDevice(name string) string {
	if strings.HasPrefix(name, "/dev/") {
		return name
	}
	return "/dev/" + name
}

// BuildMdadmCreateArgs constructs the argument list for `mdadm --create`.
func BuildMdadmCreateArgs(r RAID) []string {
	args := []string{
		"--create", normalizeRAIDDevice(r.Name),
		"--metadata=1.2",
		"--level=" + r.Level,
		"--raid-devices=" + strconv.Itoa(len(r.Devices)),
	}
	if len(r.Spare) > 0 {
		args = append(args, "--spare-devices="+strconv.Itoa(len(r.Spare)))
	}
	args = append(args, "--run", "--force")
	args = append(args, r.Devices...)
	args = append(args, r.Spare...)
	return args
}

// CreateRAID validates and assembles the RAID array via mdadm.
func CreateRAID(r RAID) error {
	if err := ValidateRAID(r); err != nil {
		return err
	}
	dev := normalizeRAIDDevice(r.Name)
	log.Infof("Creating RAID%s array %s across %v", r.Level, dev, r.Devices)

	// Named form (/dev/md/<name>) needs /dev/md/ to exist so mdadm can
	// mknod the symlink. udev normally creates this; HookOS has no udev.
	if strings.HasPrefix(dev, "/dev/md/") {
		if err := os.MkdirAll("/dev/md", 0o755); err != nil {
			return fmt.Errorf("raid: could not create /dev/md: %w", err)
		}
	}

	args := BuildMdadmCreateArgs(r)
	return runMdadm(args...)
}

// StopRAID stops an active mdadm array if present. Non-fatal if not assembled.
func StopRAID(name string) error {
	dev := normalizeRAIDDevice(name)
	if _, err := os.Stat(dev); os.IsNotExist(err) {
		return nil
	}
	log.Infof("Stopping RAID array %s", dev)
	return runMdadm("--stop", dev)
}

// ZeroSuperblock clears any mdadm superblock from a member device. Ignores
// devices without a superblock.
func ZeroSuperblock(device string) error {
	if _, err := os.Stat(device); os.IsNotExist(err) {
		return nil
	}
	log.Infof("Zeroing RAID superblock on %s", device)
	cmd := exec.Command("/sbin/mdadm", "--zero-superblock", device)
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
	// Best-effort: mdadm returns non-zero if there is no superblock to zero.
	_ = cmd.Run()
	return nil
}

func runMdadm(args ...string) error {
	cmd := exec.Command("/sbin/mdadm", args...)
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("mdadm %s: %w", strings.Join(args, " "), err)
	}
	return nil
}
