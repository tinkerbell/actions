package main

import (
	"fmt"

	"github.com/tinkerbell/hub/kexec/cmd"
)

func main() {
	fmt.Printf("KEXEC - Kernel Exec\n------------------------\n")
	cmd.Execute()
}
