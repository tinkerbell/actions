package main

import (
	"fmt"

	"github.com/tinkerbell/hub/actions/kexec/v1/cmd"
)

func main() {

	fmt.Printf("KEXEC - Kernel Exec\n------------------------\n")
	cmd.Execute()

}
