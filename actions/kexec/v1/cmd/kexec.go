package cmd

import (
	"fmt"
	"io/ioutil"
	"os"
	"strconv"
	"syscall"

	"path/filepath"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/tinkerbell/hub/actions/kexec/v1/cmd/grub"
	"github.com/tinkerbell/hub/actions/kexec/v1/pkg/esxiboot"
	"github.com/tinkerbell/hub/actions/kexec/v1/pkg/uefiboot"

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

		//Uefi configuration - more debug that you can imagine
		uefiBoot := os.Getenv("UEFI_BOOT")
		uefiboot.UEFIDebug = os.Getenv("UEFI_DEBUG")
		uefiboot.UEFIBase = os.Getenv("UEFI_BASE")
		uefiboot.UEFISerialAddr = os.Getenv("UEFI_SERIALADDR")
		uefiboot.UEFISerialWidth = os.Getenv("UEFI_SERIALWIDTH")
		uefiboot.UEFISerialHertz = os.Getenv("UEFI_SERIALHERTZ")
		uefiboot.UEFISerialBaud = os.Getenv("UEFI_SERIALBAUD")

		uefiEnable, _ := strconv.ParseBool(uefiBoot)

		if uefiEnable {
			// We never come back from here
			uefiboot.Boot()
		}

		//Esxi boot - equally as debug as you can imagine
		esxiBoot := os.Getenv("ESXI_BOOT")
		esxienable, _ := strconv.ParseBool(esxiBoot)
		if esxienable {
			// We never come back from here
			esxiboot.Boot(blockDevice, cmdLine)
		}

		// These two strings contain the updated paths including the mountAction path
		var kernelMountPath, initrdMountPath string

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

		// If we specify no kernelPath then we will fallback to autodetect and ignore the initrd and cmdline that may be passed
		// by environment variables
		if kernelPath == "" {
			grubFile, err := ioutil.ReadFile(fmt.Sprintf("%s/boot/grub/grub.cfg", mountAction))
			if err != nil {
				log.Fatal(err)
			}
			bootConfig := grub.GetDefaultConfig(string(grubFile))
			if bootConfig == nil {
				log.Fatal("No Kernel configuration passed in [KERNEL_PATH] and unable to parse [/boot/grub/grub.conf]")
			}
			log.Infof("Loaded boot config: %#v", bootConfig)
			kernelMountPath = filepath.Join(mountAction, bootConfig.Kernel)
			initrdMountPath = filepath.Join(mountAction, bootConfig.Initramfs)
			// Overwrite the cmdline with what is found in grub.conf, unless something specific is added
			if cmdLine == "" {
				cmdLine = bootConfig.KernelArgs
			}
		} else {
			kernelMountPath = filepath.Join(mountAction, kernelPath)
			initrdMountPath = filepath.Join(mountAction, initrdPath)
		}
		// /mountAction/boot/vmlinuz
		kernel, err := os.Open(kernelMountPath) // For read access.
		if err != nil {
			log.Fatal(err)
		}
		// /mountAction/boot/vmlinuz
		initrd, err := os.Open(initrdMountPath) // For read access.
		if err != nil {
			log.Fatal(err)
		}

		log.Infof("Running Kexec: kernel: %s, initrd: %s, cmdLine: %v", kernelMountPath, initrdMountPath, cmdLine)
		// Load the kernel configuration into memory
		err = unix.KexecFileLoad(int(kernel.Fd()), int(initrd.Fd()), cmdLine, 0)
		if err != nil {
			log.Fatal(err)
		}
		log.Info("Rebooting system")
		// Call the unix reboot command with the kexec functionality
		unix.Reboot(unix.LINUX_REBOOT_CMD_KEXEC)
	},
}

// Execute - starts the command parsing process
func Execute() {
	if err := kexecCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
