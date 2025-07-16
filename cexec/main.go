package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/peterbourgon/ff/v3"
)

const (
	mountAction = "/mountAction"
	envPrefix   = "CEXEC"
)

type settings struct {
	blockDevice        string
	filesystemType     string
	chroot             string
	defaultInterpreter string
	cmdLine            string
	updateResolvConf   bool
}

func main() {
	ctx, done := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGHUP, syscall.SIGTERM)
	defer done()

	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	fs := flag.NewFlagSet("cexec", flag.ExitOnError)
	s := settings{}
	fs.StringVar(&s.blockDevice, "block-device", "", "block device to mount (required)")
	fs.StringVar(&s.filesystemType, "fs-type", "", "filesystem type (required)")
	fs.StringVar(&s.chroot, "chroot", "", "use chroot environment to run given command (deprecated)")
	fs.StringVar(&s.defaultInterpreter, "default-interpreter", "", "default interpreter (optional)")
	fs.StringVar(&s.cmdLine, "cmd-line", "", "command line to execute (required)")
	fs.BoolVar(&s.updateResolvConf, "update-resolv-conf", false, "update /etc/resolv.conf in chroot environment (optional)")
	jsonLogger := fs.Bool("json-outout", true, "enable json output for logging")

	if err := ff.Parse(fs, os.Args[1:], ff.WithEnvVarNoPrefix()); err != nil {
		logger.Error(err.Error())
		os.Exit(10)
	}

	// check for required fields to be set.
	if missingFields := s.checkRequiredFields(); len(missingFields) > 0 {
		fmt.Fprintln(os.Stderr, "missing required fields", missingFields)
		fmt.Fprintln(os.Stderr)
		fs.Usage()
		os.Exit(20)
	}

	// TODO(jacobweinstock): add field validations for the settings struct.

	if *jsonLogger {
		logger = slog.New(slog.NewJSONHandler(os.Stdout, nil))
	}

	logger.Info("debugging",
		"block-device", s.blockDevice,
		"fs-type", s.filesystemType,
		"chroot", s.chroot,
		"default-interpreter", s.defaultInterpreter,
		"cmd-line", s.cmdLine,
		"json-output", *jsonLogger,
		"update-resolv-conf", s.updateResolvConf,
	)

	if err := s.cexec(ctx, logger); err != nil {
		logger.ErrorContext(ctx, err.Error())
		os.Exit(30)
	}
}

func (s settings) checkRequiredFields() []string {
	var missingFields []string
	if s.blockDevice == "" {
		missingFields = append(missingFields, "block-device")
	}
	if s.filesystemType == "" {
		missingFields = append(missingFields, "fs-type")
	}
	if s.cmdLine == "" {
		missingFields = append(missingFields, "cmd-line")
	}
	return missingFields
}

func (s settings) cexec(ctx context.Context, log *slog.Logger) error {
	log.Info("CEXEC - Chroot Exec")

	if s.blockDevice == "" {
		return errors.New("no Block Device speified with Environment Variable [BLOCK_DEVICE]")
	}

	// Create the /mountAction mountpoint (no folders exist previously in scratch container)
	if err := os.Mkdir(mountAction, os.ModeDir); err != nil {
		return fmt.Errorf("error creating the mount point [%s], error: %w", mountAction, err)
	}

	// Mount the block device to the /mountAction point
	if err := syscall.Mount(s.blockDevice, mountAction, s.filesystemType, 0, ""); err != nil {
		return fmt.Errorf("error mounting [%s] -> [%s], error: %v", s.blockDevice, mountAction, err)
	}
	defer func() {
		if err := syscall.Unmount(mountAction, 0); err != nil {
			log.Error("error unmounting device", "source", s.blockDevice, "destination", mountAction, "error", err)
		} else {
			log.Info("unmounted device successfully", "source", s.blockDevice, "destination", mountAction)
		}
	}()
	log.Info("mounted device successfully", "source", s.blockDevice, "destination", mountAction)

	if s.chroot != "" {
		if s.updateResolvConf {
			// fix resolv.conf as it normally doesn't work in chroot
			// backup the original resolv.conf
			resolv := filepath.Join(mountAction, "etc", "resolv.conf")
			if err := backupFile(resolv); err == nil {
				defer func() {
					// restore the original resolv.conf
					if err := restoreFile(resolv); err != nil {
						log.Error("unable to restore resolv.conf", "error", err)
					}
				}()
				// create an empty resolv.conf in the chroot so that it can be bind mounted
				if _, err := os.Create(resolv); err != nil {
					return fmt.Errorf("error creating resolv.conf, resolv.conf will not work in the chroot: %w", err)
				}
			} else {
				return fmt.Errorf("error backing up resolv.conf, resolv.conf will not work in the chroot: %w", err)
			}
		}

		if err := s.mountSpecialDirs(mountAction); err != nil {
			return err
		}
		defer s.umountSpecialDirs(mountAction)
		log.Info("Changing root before executing command")
		exitChroot, err := chroot(mountAction)
		if err != nil {
			return fmt.Errorf("error changing root to [%s], error: %w", mountAction, err)
		}
		defer exitChroot()
	}

	if s.defaultInterpreter != "" {
		// Split the interpreter by space, in the event that the default intprepretter has flags.
		di := strings.Split(s.defaultInterpreter, " ")
		if len(di) == 0 {
			return fmt.Errorf("error parsing [\"DEFAULT_INTERPRETER\"] [%s]", s.defaultInterpreter)
		}
		// Look for default shell intepreter
		if _, err := os.Stat(di[0]); os.IsNotExist(err) {
			return fmt.Errorf("unable to find the [\"DEFAULT_INTERPRETER\"] [%s], check chroot and interpreter path", s.defaultInterpreter)
		}
		di = append(di, s.cmdLine)
		cmd := exec.Command(di[0], di[1:]...)
		cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
		debugCMD := fmt.Sprintf("%s %v", di[0], di[1:])
		if err := cmd.Start(); err != nil {
			return fmt.Errorf("error starting [%s] [%w]", debugCMD, err)
		}
		if err := cmd.Wait(); err != nil {
			return fmt.Errorf("error running [%s] [%w]", debugCMD, err)
		}
	} else {
		// Format the cmdLine string into separate execution tasks
		commandLines := strings.Split(s.cmdLine, ";")
		for x := range commandLines {
			command := strings.Split(commandLines[x], " ")
			cmd := exec.Command(command[0], command[1:]...)
			cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
			debugCMD := fmt.Sprintf("%s %v", command[0], command[1:])
			if err := cmd.Start(); err != nil {
				return fmt.Errorf("error starting [%s] [%w]", debugCMD, err)
			}
			if err := cmd.Wait(); err != nil {
				return fmt.Errorf("error running [%s] [%w]", debugCMD, err)
			}
		}
	}

	return nil
}

// chroot handles changing the root, and returning a function to return back to the present directory.
func chroot(path string) (func() error, error) {
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

// mountSpecialDirs ensures that /dev /proc /sys /etc/resolv.conf exist in the chroot.
func (s settings) mountSpecialDirs(path string) error {
	if path == "" {
		return errors.New("mount path cannot be empty")
	}
	// Mount dev
	dev := filepath.Join(path, "dev")
	if err := syscall.Mount("/dev", dev, "", syscall.MS_BIND, ""); err != nil {
		return fmt.Errorf("couldn't mount /dev to %v: %w", dev, err)
	}

	// Mount proc
	proc := filepath.Join(path, "proc")
	if err := syscall.Mount("/proc", proc, "", syscall.MS_BIND, ""); err != nil {
		return fmt.Errorf("couldn't mount /proc to %v: %w", proc, err)
	}

	// Mount sys
	sys := filepath.Join(path, "sys")
	if err := syscall.Mount("/sys", sys, "", syscall.MS_BIND, ""); err != nil {
		return fmt.Errorf("couldn't mount /sys to %v: %w", sys, err)
	}

	if s.updateResolvConf {
		// Mount /etc/resolv.conf
		resolv := filepath.Join(path, "etc/resolv.conf")
		if err := syscall.Mount("/etc/resolv.conf", resolv, "", syscall.MS_BIND|syscall.MS_RDONLY, ""); err != nil {
			return fmt.Errorf("couldn't mount /etc/resolv.conf to %v: %w", resolv, err)
		}
	}

	return nil
}

func (s settings) umountSpecialDirs(path string) error {
	// Unmount dev
	dev := filepath.Join(path, "dev")
	if err := syscall.Unmount(dev, 0); err != nil {
		return fmt.Errorf("couldn't unmount %v: %w", dev, err)
	}

	// Unmount proc
	proc := filepath.Join(path, "proc")
	if err := syscall.Unmount(proc, 0); err != nil {
		return fmt.Errorf("couldn't unmount %v: %w", proc, err)
	}

	// Unmount sys
	sys := filepath.Join(path, "sys")
	if err := syscall.Unmount(sys, 0); err != nil {
		return fmt.Errorf("couldn't unmount %v: %w", sys, err)
	}

	if s.updateResolvConf {
		// Unmount /etc/resolv.conf
		resolv := filepath.Join(path, "etc/resolv.conf")
		if err := syscall.Unmount(resolv, 0); err != nil {
			return fmt.Errorf("couldn't unmount %v: %w", resolv, err)
		}
	}

	return nil
}

func backupFile(file string) error {
	if err := os.Rename(file, file+".backup"); err != nil {
		return fmt.Errorf("error backing up %v: %w", file, err)
	}
	return nil
}

func restoreFile(file string) error {
	if err := os.Rename(file+".backup", file); err != nil {
		return fmt.Errorf("error restoring %v: %w", file, err)
	}
	return nil
}
