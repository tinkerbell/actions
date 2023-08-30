package lvm

import (
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strings"

	log "github.com/sirupsen/logrus"
)

var lvNameRegexp = regexp.MustCompile("^[A-Za-z0-9_+.][A-Za-z0-9_+.-]*$")
var vgNameRegexp = regexp.MustCompile("^[A-Za-z0-9_+.][A-Za-z0-9_+.-]*$")
var tagRegexp = regexp.MustCompile("^[A-Za-z0-9_+.][A-Za-z0-9_+.-]*$")

type VolumeGroup struct {
	name string
}

// CreatePhysicalVolume creates a physical volume of the given device.
func CreatePhysicalVolume(dev string) error {
	if err := run("pvcreate", dev); err != nil {
		return fmt.Errorf("lvm: CreatePhysicalVolume: %v", err)
	}
	return nil
}

// PVScan runs the `pvscan --cache <dev>` command. It scans for the
// device at `dev` and adds it to the LVM metadata cache if `lvmetad`
// is running. If `dev` is an empty string, it scans all devices.
func PVScan(dev string) error {
	args := []string{"--cache"}
	if dev != "" {
		args = append(args, dev)
	}
	return run("pvscan", args...)
}

// VGScan runs the `vgscan --cache <name>` command. It scans for the
// volume group and adds it to the LVM metadata cache if `lvmetad`
// is running. If `name` is an empty string, it scans all volume groups.
func VGScan(name string) error {
	args := []string{"--cache"}
	if name != "" {
		args = append(args, name)
	}
	return run("vgscan", args...)
}

// ValidateVolumeGroupName validates a volume group name. A valid volume group
// name can consist of a limited range of characters only. The allowed
// characters are [A-Za-z0-9_+.-].
func ValidateVolumeGroupName(name string) error {
	if !vgNameRegexp.MatchString(name) {
		return fmt.Errorf("lvm: Volume group name %q contains invalid character, valid set includes: [A-Za-z0-9_+.-]", name)
	}
	return nil
}

// ValidateTag validates a tag. LVM tags are strings of up to 1024
// characters. LVM tags cannot start with a hyphen. A valid tag can consist of
// a limited range of characters only. The allowed characters are
// [A-Za-z0-9_+.-]. As of the Red Hat Enterprise Linux 6.1 release, the list of
// allowed characters was extended, and tags can contain the /, =, !, :, #, and
// & characters.
// See https://access.redhat.com/documentation/en-us/red_hat_enterprise_linux/7/html/logical_volume_manager_administration/lvm_tags
func ValidateTag(tag string) error {
	if len(tag) > 1024 {
		return fmt.Errorf("lvm: Tag %q is too long, maximum length is 1024 characters", tag)
	}
	if !tagRegexp.MatchString(tag) {
		return fmt.Errorf("lvm: Tag %q contains invalid character, valid set includes: [A-Za-z0-9_+.-]", tag)
	}
	return nil
}

// CreateVolumeGroup creates a new volume group.
func CreateVolumeGroup(name string, pvs []string, tags []string) (*VolumeGroup, error) {
	var args []string

	if err := ValidateVolumeGroupName(name); err != nil {
		return nil, err
	}

	for _, tag := range tags {
		if tag != "" {
			if err := ValidateTag(tag); err != nil {
				return nil, err
			}
			args = append(args, "--add-tag="+tag)
		}
	}

	args = append(args, name)
	for _, pv := range pvs {
		args = append(args, pv)
	}

	if err := run("vgcreate", args...); err != nil {
		return nil, err
	}

	if err := PVScan(""); err != nil {
		log.Warnf("error during pvscan: %s", err.Error())
	}

	if err := VGScan(""); err != nil {
		log.Warnf("error during vgscan: %s", err.Error())
	}
	return &VolumeGroup{name}, nil
}

// ValidateLogicalVolumeName validates a volume group name. A valid volume
// group name can consist of a limited range of characters only. The allowed
// characters are [A-Za-z0-9_+.-].
func ValidateLogicalVolumeName(name string) error {
	if !lvNameRegexp.MatchString(name) {
		return fmt.Errorf("lvm: Logical volume name %q contains invalid character, valid set includes: [A-Za-z0-9_+.-]", name)
	}

	return nil
}

// CreateLogicalVolume creates a logical volume of the given device
// and size.
//
// The actual size may be larger than asked for as the smallest
// increment is the size of an extent on the volume group in question.
//
// If sizeInBytes is zero the entire available space is allocated.
//
// Additional optional config items can be specified using CreateLogicalVolumeOpt
func (vg *VolumeGroup) CreateLogicalVolume(name string, sizeInBytes uint64, tags []string, opts []string) error {
	if err := ValidateLogicalVolumeName(name); err != nil {
		return err
	}

	// Validate the tag.
	var args []string
	for _, tag := range tags {
		if tag != "" {
			if err := ValidateTag(tag); err != nil {
				return err
			}
			args = append(args, "--add-tag="+tag)
		}
	}
	args = append(args, fmt.Sprintf("--size=%db", sizeInBytes))
	args = append(args, "--name="+name)
	args = append(args, vg.name)
	args = append(args, opts...)

	if err := run("lvcreate", args...); err != nil {
		if isInsufficientSpace(err) {
			return fmt.Errorf("lvm: not enough free space")
		}
		if isInsufficientDevices(err) {
			return fmt.Errorf("lvm: not enough underlying devices")
		}
		return err
	}
	return nil
}

func run(cmd string, extraArgs ...string) error {
	var args []string
	args = append(args, extraArgs...)
	c := exec.Command(cmd, args...)
	c.Stdout, c.Stderr = os.Stdout, os.Stderr

	return c.Run()
}

// isInsufficientSpace returns true if the error is due to insufficient space
func isInsufficientSpace(err error) bool {
	return strings.Contains(strings.ToLower(err.Error()), "insufficient free space")
}

// isInsufficientDevices returns true if the error is due to insufficient underlying devices
func isInsufficientDevices(err error) bool {
	return strings.Contains(err.Error(), "Insufficient suitable allocatable extents for logical volume")
}
