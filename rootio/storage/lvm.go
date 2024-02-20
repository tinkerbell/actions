package storage

import (
	"fmt"
	"github.com/tinkerbell/hub/rootio/lvm"
)

func CreateVolumeGroup(volumeGroup VolumeGroup) error {
	for _, p := range volumeGroup.PhysicalVolumes {
		if err := lvm.CreatePhysicalVolume(p); err != nil {
			return fmt.Errorf("failed to create physical volume %s: %v", p, err)
		}
	}

	vg, err := lvm.CreateVolumeGroup(volumeGroup.Name, volumeGroup.PhysicalVolumes, volumeGroup.Tags)
	if err != nil {
		return fmt.Errorf("failed to create volume group %s: %v", volumeGroup.Name, err)
	}

	for _, lv := range volumeGroup.LogicalVolumes {
		if err := vg.CreateLogicalVolume(lv.Name, lv.Size, lv.Tags, lv.Opts); err != nil {
			return fmt.Errorf("failed to create logical volume %s: %v", lv.Name, err)
		}
	}

	return nil
}
