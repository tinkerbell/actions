//go:build linux

package main

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"syscall"

	"github.com/tinkerbell/actions/ubootenv/ubootenv"
)

const mountAction = "/mountAction"

func main() {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	logger.Info("UBOOTENV - U-Boot Environment Variable Writer")

	blockDevice := os.Getenv("DEST_DISK")
	fsType := os.Getenv("FS_TYPE")
	envPath := os.Getenv("ENV_FILE")
	envVarsJSON := os.Getenv("ENV_VARS")

	if blockDevice == "" {
		logger.Error("DEST_DISK is required")
		os.Exit(1)
	}

	if fsType == "" {
		fsType = "vfat"
	}

	if envPath == "" {
		envPath = "/boot/uboot.env"
	}

	if envVarsJSON == "" {
		logger.Error("ENV_VARS is required (JSON object of key/value pairs)")
		os.Exit(1)
	}

	var newVars map[string]string
	if err := json.Unmarshal([]byte(envVarsJSON), &newVars); err != nil {
		logger.Error("Failed to parse ENV_VARS as JSON", "error", err)
		os.Exit(1)
	}

	if len(newVars) == 0 {
		logger.Info("No environment variables to set, nothing to do")
		return
	}

	if err := os.MkdirAll(mountAction, 0o755); err != nil {
		logger.Error("Error creating mountpoint", "path", mountAction, "error", err)
		os.Exit(1)
	}

	if err := syscall.Mount(blockDevice, mountAction, fsType, 0, ""); err != nil {
		logger.Error("Failed to mount block device", "device", blockDevice, "mountpoint", mountAction, "fstype", fsType, "error", err)
		os.Exit(1)
	}
	defer func() {
		if err := syscall.Unmount(mountAction, 0); err != nil {
			logger.Error("Error unmounting device", "source", blockDevice, "destination", mountAction, "error", err)
		} else {
			logger.Info("Unmounted device", "source", blockDevice, "destination", mountAction)
		}
	}()
	logger.Info("Mounted device", "source", blockDevice, "destination", mountAction)

	fullPath := filepath.Join(mountAction, envPath)

	data, err := os.ReadFile(fullPath)
	if err != nil {
		logger.Error("Failed to read U-Boot environment file", "path", fullPath, "error", err)
		os.Exit(1)
	}

	env, err := ubootenv.Parse(data)
	if err != nil {
		logger.Error("Failed to parse U-Boot environment", "error", err)
		os.Exit(1)
	}

	logger.Info(fmt.Sprintf("Parsed U-Boot environment: %d existing variables", len(env.Vars)))

	for k, v := range newVars {
		if v == "" {
			logger.Info("Deleting variable", "key", k)
			delete(env.Vars, k)
		} else {
			logger.Info("Setting variable", "key", k, "value", v)
			env.Vars[k] = v
		}
	}

	out, err := env.Marshal()
	if err != nil {
		logger.Error("Failed to marshal U-Boot environment", "error", err)
		os.Exit(1)
	}

	if err := os.WriteFile(fullPath, out, 0o644); err != nil {
		logger.Error("Failed to write U-Boot environment file", "path", fullPath, "error", err)
		os.Exit(1)
	}

	logger.Info("U-Boot environment updated successfully", "path", envPath, "variables_set", len(newVars))
}
