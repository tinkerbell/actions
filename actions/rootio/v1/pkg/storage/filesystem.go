package storage

import (
	"fmt"
	"os"
	"os/exec"

	"github.com/thebsdbox/rootio/pkg/types.go"
)

// FileSystemCreate handles the creation of filesystems
func FileSystemCreate(f types.Filesystem) error {
	var cmd *exec.Cmd
	var debugCMD string

	if f.Mount.Format == "swap" {
		cmd = exec.Command("/sbin/mkswap", f.Mount.Device)
		debugCMD = fmt.Sprintf("%s %s", "/sbin/mkswap", f.Mount.Device)
	} else {
		// Add filesystem flags
		f.Mount.Create.Options = append(f.Mount.Create.Options, "-t")
		f.Mount.Create.Options = append(f.Mount.Create.Options, f.Mount.Format)

		// Add force
		f.Mount.Create.Options = append(f.Mount.Create.Options, "-F")

		// Add Device to formate
		f.Mount.Create.Options = append(f.Mount.Create.Options, f.Mount.Device)

		// Format disk
		cmd = exec.Command("/sbin/mke2fs", f.Mount.Create.Options...)
		for i := range f.Mount.Create.Options {
			debugCMD = fmt.Sprintf("%s %s", debugCMD, f.Mount.Create.Options[i])
		}

	}
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr

	err := cmd.Start()
	if err != nil {
		return fmt.Errorf("Command [%s] Filesystem [%v]", debugCMD, err)
	}
	err = cmd.Wait()
	if err != nil {
		return fmt.Errorf("Command [%s] Filesystem [%v]", debugCMD, err)
	}

	return nil
}
