//go:build linux

package main

import (
	"encoding/json"
	"errors"
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

	if err := run(logger); err != nil {
		logger.Error(err.Error())
		os.Exit(1)
	}
}

func run(logger *slog.Logger) error {
	blockDevice := os.Getenv("DEST_DISK")
	fsType := os.Getenv("FS_TYPE")
	envPath := os.Getenv("ENV_FILE")
	envVarsJSON := os.Getenv("ENV_VARS")

	if blockDevice == "" {
		return errors.New("DEST_DISK is required")
	}

	if fsType == "" {
		fsType = "vfat"
	}

	if envPath == "" {
		envPath = "/boot/uboot.env"
	}

	if envVarsJSON == "" {
		return errors.New("ENV_VARS is required (JSON object of key/value pairs)")
	}

	var newVars map[string]string
	if err := json.Unmarshal([]byte(envVarsJSON), &newVars); err != nil {
		return fmt.Errorf("failed to parse ENV_VARS as JSON: %w", err)
	}

	if len(newVars) == 0 {
		logger.Info("No environment variables to set, nothing to do")
		return nil
	}

	if err := os.MkdirAll(mountAction, 0o755); err != nil {
		return fmt.Errorf("error creating mountpoint %s: %w", mountAction, err)
	}

	if err := syscall.Mount(blockDevice, mountAction, fsType, 0, ""); err != nil {
		return fmt.Errorf("failed to mount block device %s on %s as %s: %w", blockDevice, mountAction, fsType, err)
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
		return fmt.Errorf("failed to read U-Boot environment file %s: %w", fullPath, err)
	}

	env, err := ubootenv.Parse(data)
	if err != nil {
		return fmt.Errorf("failed to parse U-Boot environment: %w", err)
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
		return fmt.Errorf("failed to marshal U-Boot environment: %w", err)
	}

	if err := os.WriteFile(fullPath, out, 0o644); err != nil { //nolint:gosec // G306: the boot firmware reads this file.
		return fmt.Errorf("failed to write U-Boot environment file %s: %w", fullPath, err)
	}

	logger.Info("U-Boot environment updated successfully", "path", envPath, "variables_set", len(newVars))

	return nil
}
