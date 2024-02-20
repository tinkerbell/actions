package storage

import (
	"fmt"
	"os"
	"os/exec"
	log "github.com/sirupsen/logrus"
)

// FileSystemCreate handles the creation of filesystems.
func FileSystemCreate(f Filesystem) error {
	var cmd *exec.Cmd
	var debugCMD string

	switch f.Mount.Format {
	case "swap":
		cmd = exec.Command("/sbin/mkswap", f.Mount.Device)
		debugCMD = fmt.Sprintf("%s %s", "/sbin/mkswap", f.Mount.Device)
	case "ext4", "ext3", "ext2":
		// Add filesystem flags
		f.Mount.Create.Options = append(f.Mount.Create.Options, "-t")
		f.Mount.Create.Options = append(f.Mount.Create.Options, f.Mount.Format)

		// Add force
		f.Mount.Create.Options = append(f.Mount.Create.Options, "-F")

		// Add Device to formate
		f.Mount.Create.Options = append(f.Mount.Create.Options, f.Mount.Device)

		// Format disk
		cmd = exec.Command("/sbin/mke2fs", f.Mount.Create.Options...)

		// Build command for error message
		debugCMD := "/sbin/mke2fs"
		for i := range f.Mount.Create.Options {
			debugCMD = fmt.Sprintf("%s %s", debugCMD, f.Mount.Create.Options[i])
		}
	case "vfat":
		f.Mount.Create.Options = append(f.Mount.Create.Options, f.Mount.Device)
		cmd = exec.Command("/sbin/mkfs.fat", f.Mount.Create.Options...)

		// Build command for error message
		debugCMD := "/sbin/mkfs.fat"
		for i := range f.Mount.Create.Options {
			debugCMD = fmt.Sprintf("%s %s", debugCMD, f.Mount.Create.Options[i])
		}
	case "xfs":
		// Add force
		f.Mount.Create.Options = append(f.Mount.Create.Options, "-f")

		// Add Device to formate
		f.Mount.Create.Options = append(f.Mount.Create.Options, f.Mount.Device)

		// Format disk
		cmd = exec.Command("/sbin/mkfs.xfs", f.Mount.Create.Options...)

		// Build command for error message
		debugCMD := "/sbin/mkfs.xfs"
		for i := range f.Mount.Create.Options {
			debugCMD = fmt.Sprintf("%s %s", debugCMD, f.Mount.Create.Options[i])
		}
	default:
		log.Warnf("unknown filesystem type [%s]", f.Mount.Format)
	}
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr

	err := cmd.Start()
	if err != nil {
		return fmt.Errorf("command [%s] Filesystem [%w]", debugCMD, err)
	}
	err = cmd.Wait()
	if err != nil {
		return fmt.Errorf("command [%s] Filesystem [%w]", debugCMD, err)
	}

	return nil
}
