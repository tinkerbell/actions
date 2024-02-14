package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"

	log "github.com/sirupsen/logrus"
)

const mountAction = "/mountAction"

func main() {
	fmt.Printf("CEXEC - Chroot Exec\n------------------------\n")

	// Parse the environment variables that are passed into the action
	blockDevice := os.Getenv("BLOCK_DEVICE")
	filesystemType := os.Getenv("FS_TYPE")
	chroot := os.Getenv("CHROOT")
	defaultInterpreter := os.Getenv("DEFAULT_INTERPRETER")
	cmdLine := os.Getenv("CMD_LINE")

	var exitChroot func() error

	if blockDevice == "" {
		log.Fatalf("No Block Device speified with Environment Variable [BLOCK_DEVICE]")
	}

	// Create the /mountAction mountpoint (no folders exist previously in scratch container)
	err := os.Mkdir(mountAction, os.ModeDir)
	if err != nil {
		log.Fatalf("Error creating the action Mountpoint [%s]", mountAction)
	}

	// Mount the block device to the /mountAction point
	err = syscall.Mount(blockDevice, mountAction, filesystemType, 0, "")
	if err != nil {
		log.Fatalf("Mounting [%s] -> [%s] error [%v]", blockDevice, mountAction, err)
	}
	log.Infof("Mounted [%s] -> [%s]", blockDevice, mountAction)

	if chroot != "" {
		err = MountSpecialDirs()
		if err != nil {
			log.Fatal(err)
		}
		log.Infoln("Changing root before executing command")
		exitChroot, err = Chroot(mountAction)
		if err != nil {
			log.Fatalf("Error changing root to [%s]", mountAction)
		}
		// Change /dev/null file permission to 0666
		err := os.Chmod("/dev/null", 0666)
		if err != nil {
			fmt.Println("Error occurred while changing /dev/null to 0666:", err)
			return
		}
	}

	if defaultInterpreter != "" {
		// Split the interpreter by space, in the event that the default intprepretter has flags.
		di := strings.Split(defaultInterpreter, " ")
		if len(di) == 0 {
			log.Fatalf("Error parsing [\"DEFAULT_INTERPETER\"] [%s]\n", defaultInterpreter)
		}
		// Look for default shell intepreter
		_, err = os.Stat(di[0])
		if os.IsNotExist(err) {
			log.Fatalf("Unable to find the [\"DEFAULT_INTERPETER\"] [%s], check chroot and interpreter path", defaultInterpreter)
		}
		di = append(di, cmdLine)
		cmd := exec.Command(di[0], di[1:]...)
		cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
		debugCMD := fmt.Sprintf("%s %v", di[0], di[1:])
		err = cmd.Start()
		if err != nil {
			log.Fatalf("Error starting [%s] [%v]", debugCMD, err)
		}
		err = cmd.Wait()
		if err != nil {
			log.Fatalf("Error running [%s] [%v]", debugCMD, err)
		}
	} else {
		// Format the cmdLine string into separate execution tasks
		commandLines := strings.Split(cmdLine, ";")
		for x := range commandLines {
			command := strings.Split(commandLines[x], " ")
			cmd := exec.Command(command[0], command[1:]...)
			cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
			debugCMD := fmt.Sprintf("%s %v", command[0], command[1:])
			err = cmd.Start()
			if err != nil {
				log.Fatalf("Error starting [%s] [%v]", debugCMD, err)
			}
			err = cmd.Wait()
			if err != nil {
				log.Fatalf("Error running [%s] [%v]", debugCMD, err)
			}
		}
	}

	if chroot != "" {
		err = exitChroot()
		if err != nil {
			log.Errorf("Error exiting root from [%s], execution continuing", mountAction)
		}
	}
}

// Chroot handles changing the root, and returning a function to return back to the present directory.
func Chroot(path string) (func() error, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return nil, err
	}
	root, err := os.Open(cwd)
	if err != nil {
		return nil, err
	}

	if err := syscall.Chroot(path); err != nil {
		root.Close()
		return nil, err
	}

	// set the working directory inside container
	if err := syscall.Chdir("/"); err != nil {
		root.Close()
		return nil, err
	}

	return func() error {
		defer root.Close()
		if err := root.Chdir(); err != nil {
			return err
		}
		return syscall.Chroot(".")
	}, nil
}

// MountSpecialDirs ensures that /dev /proc /sys /dev/pts exist in the chroot.
func MountSpecialDirs() error {
	// Mount dev in RW mode to be able to fix /dev/null permissions
	dev := filepath.Join(mountAction, "dev")

	if err := syscall.Mount("none", dev, "devtmpfs", uintptr(0), ""); err != nil {
		return fmt.Errorf("couldn't mount /dev to %v: %w", dev, err)
	}

	// Mount proc
	proc := filepath.Join(mountAction, "proc")

	if err := syscall.Mount("none", proc, "proc", syscall.MS_RDONLY, ""); err != nil {
		return fmt.Errorf("couldn't mount /proc to %v: %w", proc, err)
	}

	// Mount sys
	sys := filepath.Join(mountAction, "sys")

	if err := syscall.Mount("none", sys, "sysfs", syscall.MS_RDONLY, ""); err != nil {
		return fmt.Errorf("couldn't mount /sys to %v: %w", sys, err)
	}

	// Mount /dev/pts
	devPts := filepath.Join(mountAction, "dev/pts")

	if err := syscall.Mount("none", devPts, "devpts", syscall.MS_NOSUID|syscall.MS_NOEXEC, ""); err != nil {
		return fmt.Errorf("couldn't mount /dev/pts to %v: %w", sys, err)
	}

	return nil
}
