package main

import (
	"fmt"

	"github.com/tinkerbell/hub/actions/rootio/v1/cmd"
)

func main() {
	fmt.Printf("ROOTIO - Disk Manager\n------------------------\n")
	fmt.Println("Parsing MetaData")
	cmd.Execute()
}
