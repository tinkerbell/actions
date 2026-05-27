package storage

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"syscall"

	log "github.com/sirupsen/logrus"
)

// Mount -=.
func Mount(f Filesystem) error {
	if f.Mount.Format == "swap" {
		// Format disk
		cmd := exec.CommandContext(context.Background(), "/sbin/swapon", f.Mount.Device)
		cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
		debugCMD := fmt.Sprintf("%s %s", "/sbin/swapon", f.Mount.Device)
		err := cmd.Start()
		if err != nil {
			return fmt.Errorf("command [%s] Filesystem [%w]", debugCMD, err)
		}
		err = cmd.Wait()
		if err != nil {
			return fmt.Errorf("command [%s] Filesystem [%w]", debugCMD, err)
		}
	} else {
		err := syscall.Mount(f.Mount.Device, f.Mount.Point, f.Mount.Format, 0, "")
		if err != nil {
			return fmt.Errorf("mounting [%s] -> [%s] error [%w]", f.Mount.Device, f.Mount.Point, err)
		}
	}
	log.Infof("mounted [%s] -> [%s]", f.Mount.Device, f.Mount.Point)

	return nil
}
