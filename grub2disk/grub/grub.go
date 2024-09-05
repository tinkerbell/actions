package grub

import (
	"fmt"
	"os"
	"os/exec"
	"syscall"

	log "github.com/sirupsen/logrus"
)

func makeDirAndMount(devicePath string, mountPoint string, filesystemType string) (err error) {
	if err := os.MkdirAll(mountPoint, os.ModeDir); err != nil {
		return fmt.Errorf("failed to create the directory %s: %w", mountPoint, err)
	}

	// if err = syscall.Mount(devicePath, mountPoint, filesystemType, flag, ""); err != nil {
	var params string
	if filesystemType != "" {
		params = "-t " + filesystemType
	} else {
		params = "--bind"
	}
	cmd := fmt.Sprintf("/bin/mount %s %s %s", params, devicePath, mountPoint)
	log.Infof(cmd)
	if _, err = exec.Command("/bin/sh", "-c", cmd).Output(); err != nil {
		return err
	}
	return
}

func doChroot(chrootPath string) (err error) {
	for _, dir := range []string{"/proc", "/mnt", "/dev", "/sys", "/etc", "/bin", "/sbin", "/var", "/usr", "/lib", "/tmp"} {
		mountPath := chrootPath + dir
		if err = makeDirAndMount(dir, mountPath, ""); err != nil {
			return err
		}
	}
	log.Infof("mount operation on system files completed")

	if err = syscall.Chroot(chrootPath); err != nil {
		return err
	}
	return
}

func execGrub(path string) (err error) {
	if _, err = exec.Command("/bin/sh", "-c", "/usr/sbin/grub-install "+path).Output(); err != nil {
		log.Error(fmt.Errorf("failed to execute grub-install on %s", path))
		return err
	}
	if _, err = exec.Command("/bin/sh", "-c", "/usr/sbin/grub-mkconfig -o /boot/grub/grub.cfg").Output(); err != nil {
		log.Error("failed to make grub config file")
		return err
	}
	return
}

func MountGrub(grubInstallPath string, grubBlockDevice string, filesystemType string) (err error) {
	tempMountPoint := "/mnt/chrootdir"
	if err = makeDirAndMount(grubBlockDevice, tempMountPoint, filesystemType); err != nil {
		log.Error(fmt.Errorf("failed to mount %s on %s ", grubBlockDevice, tempMountPoint))
		return err
	}
	log.Infof("mounted [%s] -> [%s]", grubBlockDevice, tempMountPoint)

	if err = doChroot(tempMountPoint); err != nil {
		log.Error(fmt.Errorf("failed to chroot %s ", tempMountPoint))
		return err
	}
	log.Infof("chroot step is completed successfully.")

	outPut, err := exec.Command("/bin/sh", "-c", "ls -lrt /mnt/chrootdir/").Output()
	if err != nil {
		log.Error(fmt.Errorf("failed to get output %s", outPut))
		return err
	}
	log.Infof(fmt.Sprint(string(outPut)))

	if err = execGrub(grubInstallPath); err != nil {
		return err
	}
	log.Infof("grub-install completed successfully.")
	return
}
