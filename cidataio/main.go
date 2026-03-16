package main

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

const (
	configISOLabel          = "cidata"
	configNetworkConfigPath = "network-config"
	configMetaDataPath      = "meta-data"
	configUserDataPath      = "user-data"
)

// run is a helper to run a shell command and log it.
func run(cmdStr string, args ...string) {
	log.Printf("Running: %s %s", cmdStr, strings.Join(args, " "))
	cmd := exec.Command(cmdStr, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		log.Fatalf("Command failed: %v", err)
	}
}

// runWithOutput runs a command and returns its stdout.
func runWithOutput(cmdStr string, args ...string) string {
	log.Printf("Running (for output): %s %s", cmdStr, strings.Join(args, " "))
	out, err := exec.Command(cmdStr, args...).CombinedOutput()
	if err != nil {
		log.Printf("Command failed: %s - %v", string(out), err)
		// Don't fatalf, as some commands (like ls) might fail gracefully
	}
	return strings.TrimSpace(string(out))
}

// findNewPartition compares a list of partitions before and after an operation.
func findNewPartition(before, after string) string {
	beforeSet := make(map[string]bool)
	for _, p := range strings.Split(before, "\n") {
		if p != "" {
			beforeSet[p] = true
		}
	}

	for _, p := range strings.Split(after, "\n") {
		if p != "" && !beforeSet[p] {
			return p // Found the new one
		}
	}
	return ""
}

// writeFileIfEnv writes content from an env var to a file.
func writeFileIfEnv(envVar, path string) {
	content := os.Getenv(envVar)
	if content == "" {
		log.Printf("Env var %s not set, skipping file.", envVar)
		return
	}

	log.Printf("Writing content from %s to %s", envVar, path)
	err := os.WriteFile(path, []byte(content), 0644)
	if err != nil {
		log.Fatalf("Failed to write file %s: %v", path, err)
	}
}

func main() {
	log.Println("Starting cidataio action...")

	// 1. Get DEST_DISK
	disk := os.Getenv("DEST_DISK")
	if disk == "" {
		log.Fatalf("DEST_DISK environment variable not set.")
	}

	// 2. Force kernel to read partition table and get "before" list
	run("partprobe", disk)
	time.Sleep(1 * time.Second) // Give udev time to create devices

	// List all partitions for this disk using regex to match both:
	// - Standard devices: /dev/sda1, /dev/sdb2, /dev/vda3
	// - NVMe/MMC devices: /dev/nvme0n1p1, /dev/mmcblk0p2
	globPattern := fmt.Sprintf("ls -1 %s* 2>/dev/null | grep -E '%sp?[0-9]+$' || true", disk, disk)
	partsBefore := runWithOutput("sh", "-c", globPattern)

	// 3. Create the new partition
	log.Printf("Creating new partition on %s", disk)
	run("sgdisk", "-n", "0:0:+2M", "-t", "0:0700", disk)

	// 4. Force kernel to re-read and find the new partition
	run("partprobe", disk)
	time.Sleep(2 * time.Second) // Give udev time to settle
	partsAfter := runWithOutput("sh", "-c", globPattern)

	newPart := findNewPartition(partsBefore, partsAfter)
	if newPart == "" {
		log.Fatalf("Could not find a new partition. Before: [%s], After: [%s]", partsBefore, partsAfter)
	}
	log.Printf("Found new partition: %s", newPart)

	// 5. Format the new partition
	log.Printf("Formatting %s as vfat with label cidata", newPart)
	run("mkfs.vfat", "-n", configISOLabel, newPart)

	// 6. Mount, Write, Unmount
	mountPoint := "/mnt/cidata"
	log.Printf("Mounting %s to %s", newPart, mountPoint)
	run("mkdir", "-p", mountPoint)
	run("mount", newPart, mountPoint)

	// 7. Write data from Env Vars
	writeFileIfEnv("USER_DATA", filepath.Join(mountPoint, configUserDataPath))
	writeFileIfEnv("META_DATA", filepath.Join(mountPoint, configMetaDataPath))
	writeFileIfEnv("NETWORK_CONFIG", filepath.Join(mountPoint, configNetworkConfigPath))

	// 8. Unmount
	log.Printf("Unmounting %s", mountPoint)
	run("umount", mountPoint)

	log.Println("cidataio action completed successfully.")
}