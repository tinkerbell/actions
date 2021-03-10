package main

import (
	"fmt"
	"log"
	"os/exec"
	"runtime"
)

// Dont have global variable !

func main() {
	// Checking the machine system
	if runtime.GOOS == "linux" {
		LinuxReboot()
	} else if runtime.GOOS == "windows" {
		WindowsReboot()
	} else {
		fmt.Println("Sorry, this action cannot run on this server!")
	}
}

// WindowsReboot is for windows
func WindowsReboot() {
	output, err := exec.Command("reboot").Output()
	// Handel error
	if err != nil {
		log.Fatalln("Some thing went wrong:", err)
	}
	// Reboot system
	result := string(output)
	fmt.Println(result)
}

// LinuxReboot is for linux
func LinuxReboot() {
	output, err := exec.Command("reboot -f Now").Output()
	if err != nil {
		log.Fatalln("Some thing went wrong:", err)
	}
	result := string(output)
	fmt.Println(result)
}
