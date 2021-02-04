package cmd

import (
	"fmt"
	"os"
	"syscall"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"golang.org/x/sys/unix"
)

// Release - this struct contains the release information populated when building kexec
var Release struct {
	Version string
	Build   string
}

const mountAction = "/mountAction"

var kexecCmd = &cobra.Command{
	Use:   "kexec",
	Short: "This is an action for performing a kexec into a new kernel/ramdisk",
	Run: func(cmd *cobra.Command, args []string) {

		// Parse the environment variables that are passed into the action
		blockDevice := os.Getenv("BLOCK_DEVICE")
		filesystemType := os.Getenv("FS_TYPE")
		kernelPath := os.Getenv("KERNEL_PATH")
		initrdPath := os.Getenv("INITRD_PATH")
		cmdLine := os.Getenv("CMD_LINE")

		if blockDevice == "" {
			log.Fatalf("No Block Device speified with Environment Variable [BLOCK_DEVICE]")
		}

		// Create the /mountAction mountpoint (no folders exist previously in scratch container)
		err := os.Mkdir(mountAction, os.ModeDir)
		if err != nil {
			log.Fatalf("Error creating the action Mountpoint [%s]", mountAction)
		}

		err = syscall.Mount(blockDevice, mountAction, filesystemType, 0, "")
		if err != nil {
			log.Fatalf("Mounting [%s] -> [%s] error [%v]", blockDevice, mountAction, err)
		}
		// /mountAction/boot/vmlinuz
		kernel, err := os.Open(fmt.Sprintf("%s%s", mountAction, kernelPath)) // For read access.
		if err != nil {
			log.Fatal(err)
		}
		// /mountAction/boot/vmlinuz
		initrd, err := os.Open(fmt.Sprintf("%s%s", mountAction, initrdPath)) // For read access.
		if err != nil {
			log.Fatal(err)
		}

		err = unix.KexecFileLoad(int(kernel.Fd()), int(initrd.Fd()), cmdLine, 0)
		if err != nil {
			log.Fatal(err)
		}
		unix.Reboot(unix.LINUX_REBOOT_CMD_KEXEC)
	},
}

func init() {

	// TODO - refactor at a later date.
}

// Execute - starts the command parsing process
func Execute() {
	if err := kexecCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
