package storage

import (
	"fmt"
	"os"
	"os/exec"
	"syscall"

	log "github.com/sirupsen/logrus"
	"github.com/tinkerbell/hub/actions/rootio/v1/pkg/types.go"
)

// Mount -=
func Mount(f types.Filesystem) error {
	if f.Mount.Format == "swap" {
		// Format disk
		cmd := exec.Command("/sbin/swapon", f.Mount.Device)
		cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
		debugCMD := fmt.Sprintf("%s %s", "/sbin/swapon", f.Mount.Device)
		err := cmd.Start()
		if err != nil {
			return fmt.Errorf("Command [%s] Filesystem [%v]", debugCMD, err)
		}
		err = cmd.Wait()
		if err != nil {
			return fmt.Errorf("Command [%s] Filesystem [%v]", debugCMD, err)
		}
	} else {
		err := syscall.Mount(f.Mount.Device, f.Mount.Point, f.Mount.Format, 0, "")
		if err != nil {
			return fmt.Errorf("Mounting [%s] -> [%s] error [%v]", f.Mount.Device, f.Mount.Point, err)
		}
	}
	log.Infof("Mounted [%s] -> [%s]", f.Mount.Device, f.Mount.Point)

	return nil
}
