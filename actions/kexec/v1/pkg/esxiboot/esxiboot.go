package esxiboot

import (
	"log"
	"os"

	"github.com/u-root/u-root/pkg/boot"
	"github.com/u-root/u-root/pkg/boot/esxi"
	"github.com/u-root/u-root/pkg/mount"
)

func Boot(disk, cmdline string) {
	//Create a temporary directory within the environment
	err := os.Mkdir("/tmp", 0755)
	if err != nil {
		log.Fatal(err)
	}

	imgs, mps, err := esxi.LoadDisk(disk)
	if err != nil {
		log.Fatalf("Failed to load ESXi configuration: %v", err)
	}

	loaded := false
	for _, img := range imgs {
		if len(cmdline) > 0 {
			img.Cmdline = img.Cmdline + cmdline
		}
		if err := img.Load(false); err != nil {
			log.Printf("Failed to load ESXi image (%v) into memory: %v", img, err)
		} else {
			log.Printf("Loaded image: %v", img)
			// We loaded one, that's it.
			loaded = true
			break
		}
	}
	for _, mp := range mps {
		if err := mp.Unmount(mount.MNT_DETACH); err != nil {
			log.Printf("Failed to unmount %s: %v", mp, err)
		}
	}
	if !loaded {
		log.Fatalf("Failed to load all ESXi images found.")
	}

	if err := boot.Execute(); err != nil {
		log.Fatalf("Failed to boot image: %v", err)
	}

}
