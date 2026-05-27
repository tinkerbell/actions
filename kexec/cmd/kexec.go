package cmd

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"syscall"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/tinkerbell/actions/kexec/cmd/grub"
	"golang.org/x/sys/unix"
)

// Release - this struct contains the release information populated when building kexec.
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
		grubCfgPath := os.Getenv("GRUBCFG_PATH")

		var kernelMountPath string
		var initrdMountPaths []string

		if blockDevice == "" {
			log.Fatalf("No Block Device speified with Environment Variable [BLOCK_DEVICE]")
		}

		if grubCfgPath == "" {
			grubCfgPath = "boot/grub/grub.cfg"
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
			grubFile, err := os.ReadFile(fmt.Sprintf("%s/%s", mountAction, grubCfgPath))
			if err != nil {
				log.Fatal(err)
			}
			bootConfig := grub.GetDefaultConfig(string(grubFile))
			if bootConfig == nil {
				log.Fatalf("No Kernel configuration passed in [KERNEL_PATH] and unable to parse [/%s]", grubCfgPath)
			}
			log.Infof("Loaded boot config: %#v", bootConfig)
			kernelMountPath = filepath.Join(mountAction, bootConfig.Kernel)
			for _, p := range bootConfig.Initramfs {
				initrdMountPaths = append(initrdMountPaths, filepath.Join(mountAction, p))
			}
			// Overwrite the cmdline with what is found in grub.conf, unless something specific is added
			if cmdLine == "" {
				cmdLine = bootConfig.KernelArgs
			}
		} else {
			kernelMountPath = filepath.Join(mountAction, kernelPath)
			initrdMountPaths = []string{filepath.Join(mountAction, initrdPath)}
		}

		kernel, err := os.Open(kernelMountPath)
		if err != nil {
			log.Fatal(err)
		}
		defer kernel.Close()

		var initrdFd int
		var kexecFlags int
		if len(initrdMountPaths) == 0 {
			log.Warn("No initrd paths found, proceeding without initrd")
			kexecFlags |= unix.KEXEC_FILE_NO_INITRAMFS
		} else {
			initrd, cleanup, err := concatFiles(initrdMountPaths)
			if err != nil {
				log.Fatal(err)
			}
			defer initrd.Close()
			defer cleanup()
			initrdFd = int(initrd.Fd())
		}

		log.Infof("Running Kexec: kernel: %s, initrd: %v, cmdLine: %v", kernelMountPath, initrdMountPaths, cmdLine)
		// Load the kernel configuration into memory
		err = unix.KexecFileLoad(int(kernel.Fd()), initrdFd, cmdLine, kexecFlags)
		if err != nil {
			log.Fatal(err)
		}
		log.Info("Rebooting system")
		// Call the unix reboot command with the kexec functionality
		_ = unix.Reboot(unix.LINUX_REBOOT_CMD_KEXEC)
	},
}

// Execute - starts the command parsing process.
func Execute() {
	if err := kexecCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

// concatFiles returns a file ready for KexecFileLoad. For a single path it
// opens the file directly; for multiple paths it concatenates them into a
// temp file (the Linux kernel accepts concatenated cpio archives).
func concatFiles(paths []string) (*os.File, func(), error) {
	if len(paths) == 1 {
		f, err := os.Open(paths[0])
		return f, func() {}, err
	}
	if err := os.MkdirAll("/tmp", 0o755); err != nil {
		return nil, func() {}, fmt.Errorf("creating /tmp: %w", err)
	}
	tmp, err := os.CreateTemp("/tmp", "initrd-*")
	if err != nil {
		return nil, func() {}, fmt.Errorf("creating temp initrd: %w", err)
	}
	for _, p := range paths {
		f, err := os.Open(p)
		if err != nil {
			tmp.Close()
			os.Remove(tmp.Name())
			return nil, func() {}, fmt.Errorf("opening initrd %s: %w", p, err)
		}
		_, err = io.Copy(tmp, f)
		f.Close()
		if err != nil {
			tmp.Close()
			os.Remove(tmp.Name())
			return nil, func() {}, fmt.Errorf("copying initrd %s: %w", p, err)
		}
	}
	name := tmp.Name()
	tmp.Close()
	// Reopen read-only — KexecFileLoad rejects files with writable fds open (ETXTBSY).
	f, err := os.Open(name)
	if err != nil {
		os.Remove(name)
		return nil, func() {}, fmt.Errorf("reopening temp initrd %s: %w", name, err)
	}
	return f, func() { os.Remove(name) }, nil
}
