package main

import (
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"syscall"

	"github.com/tinkerbell/actions/taloscmdline/efipart"
	"github.com/tinkerbell/actions/taloscmdline/kargs"
	"github.com/tinkerbell/actions/taloscmdline/uki"
)

const (
	mountAction       = "/mountAction"
	efiLabel          = "EFI"
	efiFilesystem     = "vfat"
	defaultUKIPattern = "EFI/Linux/Talos-*.efi"
)

func main() {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	logger.Info("TALOSCMDLINE - Set kernel arguments in Talos unified kernel images")

	if err := run(logger); err != nil {
		logger.Error(err.Error())
		os.Exit(1)
	}
}

func run(logger *slog.Logger) error {
	disk := os.Getenv("DEST_DISK")
	if disk == "" {
		return errors.New("no block device specified with environment variable [DEST_DISK]")
	}

	args := strings.TrimSpace(os.Getenv("KERNEL_ARGS"))
	if args == "" {
		return errors.New("no kernel arguments specified with environment variable [KERNEL_ARGS]")
	}

	pattern := os.Getenv("UKI_PATTERN")
	if pattern == "" {
		pattern = defaultUKIPattern
	}

	if filepath.IsAbs(pattern) || strings.Contains(pattern, "..") {
		return fmt.Errorf("UKI_PATTERN must be relative to the EFI partition, got %q", pattern)
	}

	// The error can be ignored: anything unparsable means false.
	strip, _ := strconv.ParseBool(os.Getenv("STRIP_SIGNATURE"))

	partition, err := efipart.Locate(disk, efiLabel)
	if err != nil {
		return err
	}

	device := efipart.Device(disk, partition.Index)
	logger.Info("Found EFI partition", "disk", disk, "partition", partition.Index, "device", device, "offset", partition.Offset, "size", partition.Size)

	if err := os.MkdirAll(mountAction, 0o755); err != nil {
		return fmt.Errorf("creating mountpoint %s: %w", mountAction, err)
	}

	if err := syscall.Mount(device, mountAction, efiFilesystem, 0, ""); err != nil {
		return fmt.Errorf("mounting %s on %s as %s: %w", device, mountAction, efiFilesystem, err)
	}

	defer func() {
		if err := syscall.Unmount(mountAction, 0); err != nil {
			logger.Error("Error unmounting device", "source", device, "destination", mountAction, "error", err)
		} else {
			logger.Info("Unmounted device", "source", device, "destination", mountAction)
		}
	}()
	logger.Info("Mounted device", "source", device, "destination", mountAction)

	updated, unchanged, err := editUKIs(mountAction, pattern, args, strip, logger)
	if err != nil {
		return err
	}

	logger.Info("Finished", "updated", updated, "unchanged", unchanged)

	return nil
}

// editUKIs merges args into the command line of every UKI under root that
// matches pattern. Files whose command line already contains the arguments
// are left untouched.
func editUKIs(root, pattern, args string, strip bool, logger *slog.Logger) (updated, unchanged int, err error) {
	matches, err := filepath.Glob(filepath.Join(root, pattern))
	if err != nil {
		return 0, 0, fmt.Errorf("invalid UKI pattern %q: %w", pattern, err)
	}

	if len(matches) == 0 {
		return 0, 0, fmt.Errorf("no UKI matching %q found on the EFI partition", pattern)
	}

	sort.Strings(matches)

	for _, path := range matches {
		rel, _ := filepath.Rel(root, path)

		info, err := uki.Inspect(path)
		if err != nil {
			return updated, unchanged, err
		}

		merged := make([]string, len(info.Cmdlines))
		changed := false

		for i, current := range info.Cmdlines {
			merged[i] = kargs.Merge(current, args)
			if merged[i] != current {
				changed = true
			}
		}

		for i, loc := range info.Locations {
			logger.Info("Found kernel command line", "uki", rel, "cmdline", i, "section", loc.Section, "offset", fmt.Sprintf("%#x", loc.Offset), "bytes", loc.Size)
		}

		if !changed {
			logger.Info("UKI already has the requested arguments", "uki", rel, "cmdline", info.Cmdlines[0])
			unchanged++

			continue
		}

		if info.Signed {
			if !strip {
				return updated, unchanged, fmt.Errorf("%s: %w; set STRIP_SIGNATURE=true to remove it (the UKI will then only boot with Secure Boot disabled)", rel, uki.ErrSigned)
			}

			logger.Warn("Removing Authenticode signature; the UKI will not boot with Secure Boot enabled", "uki", rel)
		}

		if err := uki.Rewrite(path, merged, uki.Options{StripSignature: strip}); err != nil {
			return updated, unchanged, err
		}

		for i := range merged {
			logger.Info("Updated UKI", "uki", rel, "cmdline", i, "old", info.Cmdlines[i], "new", merged[i])
		}

		updated++
	}

	return updated, unchanged, nil
}
