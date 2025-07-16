package main

import (
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"

	log "github.com/sirupsen/logrus"
)

const mountAction = "/mountAction"

func main() {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	logger.Info("WriteFile - Write file to a disk device")

	blockDevice := os.Getenv("DEST_DISK")
	filesystemType := os.Getenv("FS_TYPE")
	filePath := os.Getenv("DEST_PATH")

	contents := os.Getenv("CONTENTS")
	uid := os.Getenv("UID")
	gid := os.Getenv("GID")
	mode := os.Getenv("MODE")
	dirMode := os.Getenv("DIRMODE")

	// Validate inputs
	if blockDevice == "" {
		logger.Error("No Block Device speified with Environment Variable [DEST_DISK]")
		os.Exit(1)
	}

	if !filepath.IsAbs(filePath) {
		logger.Error("Provide path must be an absolute path")
		os.Exit(1)
	}

	modePrime, err := strconv.ParseUint(mode, 8, 32)
	if err != nil {
		logger.Error("Could not parse mode", "error", err)
		os.Exit(1)
	}

	fileMode := os.FileMode(modePrime)

	dirModePrime, err := strconv.ParseUint(dirMode, 8, 32)
	if err != nil {
		logger.Error("Could not parse dirmode", "error", err)
		os.Exit(1)
	}

	newDirMode := os.FileMode(dirModePrime)

	fileUID, err := strconv.Atoi(uid)
	if err != nil {
		logger.Error("Could not parse uid", "error", err)
		os.Exit(1)
	}

	fileGID, err := strconv.Atoi(gid)
	if err != nil {
		logger.Error("Could not parse gid", "error", err)
		os.Exit(1)
	}

	dirPath, fileName := filepath.Split(filePath)
	if len(fileName) == 0 {
		logger.Error("Provide path must include a file component")
		os.Exit(1)
	}

	// Create the /mountAction mountpoint (no folders exist previously in scratch container)
	if err := os.Mkdir(mountAction, os.ModeDir); err != nil {
		logger.Error("Error creating the action Mountpoint", "mountAction", mountAction, "error", err)
		os.Exit(1)
	}

	// Mount the block device to the /mountAction point
	if err := syscall.Mount(blockDevice, mountAction, filesystemType, 0, ""); err != nil {
		logger.Error("Mounting block device", "blockDevice", blockDevice, "mountAction", mountAction, "error", err)
		os.Exit(1)
	}
	defer func() {
		if err := syscall.Unmount(mountAction, 0); err != nil {
			logger.Error("Error unmounting device", "source", blockDevice, "destination", mountAction, "error", err)
		} else {
			logger.Info("Unmounted device successfully", "source", blockDevice, "destination", mountAction)
		}
	}()
	logger.Info("Mounted device successfully", "source", blockDevice, "destination", mountAction)

	if err := recursiveEnsureDir(mountAction, dirPath, newDirMode, fileUID, fileGID); err != nil {
		logger.Error("Failed to ensure directory exists", "error", err)
		os.Exit(1)
	}

	fqFilePath := filepath.Join(mountAction, filePath)
	// Write the file to disk
	if err := os.WriteFile(fqFilePath, []byte(contents), fileMode); err != nil {
		logger.Error("Could not write file", "filePath", filePath, "error", err)
		os.Exit(1)
	}

	if err := os.Chown(fqFilePath, fileUID, fileGID); err != nil {
		logger.Error("Could not modify ownership of file", "filePath", filePath, "error", err)
		os.Exit(1)
	}

	logger.Info("Successfully wrote file", "filePath", filePath, "blockDevice", blockDevice)
}

func dirExists(mountPath, path string) (bool, error) {
	fqPath := filepath.Join(mountPath, path)
	info, err := os.Stat(fqPath)

	switch {
	// Any error that does not indicate the directory doesn't exist
	case err != nil && !os.IsNotExist(err):
		return false, fmt.Errorf("failed to stat path %s: %w", path, err)
	// The directory already exists
	case err == nil:
		if !info.IsDir() {
			return false, fmt.Errorf("expected %s to be a path, but it is a file", path)
		}
	}

	return !os.IsNotExist(err), nil
}

func recursiveEnsureDir(mountPath, path string, mode os.FileMode, uid, gid int) error {
	// Does the directory already exist? If so we can return early
	exists, err := dirExists(mountPath, path)
	if err != nil {
		return err
	}

	if exists {
		return nil
	}

	pathParts := strings.Split(path, string(os.PathSeparator))
	if len(pathParts) == 1 && pathParts[0] == path {
		return errors.New("bad path")
	}

	basePath := string(os.PathSeparator)
	for _, part := range pathParts {
		basePath = filepath.Join(basePath, part)
		if err := ensureDir(mountPath, basePath, mode, uid, gid); err != nil {
			return err
		}
	}

	return nil
}

func ensureDir(mountPath, path string, mode os.FileMode, uid, gid int) error {
	exists, err := dirExists(mountPath, path)
	if err != nil {
		return err
	}

	if exists {
		return nil
	}

	// The directory doesn't exist, let's create it.
	fqPath := filepath.Join(mountPath, path)

	if err := os.Mkdir(fqPath, mode); err != nil {
		return fmt.Errorf("failed to create directory %s: %w", path, err)
	}

	log.Infof("Successfully created directory: %s", path)

	if err := os.Chown(fqPath, uid, gid); err != nil {
		return fmt.Errorf("failed to set ownership of directory %s to %d:%d: %w", path, uid, gid, err)
	}

	log.Infof("Successfully set ownernership of directory %s to %d:%d", path, uid, gid)

	return nil
}
