package main

import (
	"fmt"

	"github.com/tinkerbell/actions/rootio/cmd"
)

func main() {
	fmt.Printf("ROOTIO - Disk Manager\n------------------------\n")
	fmt.Println("Parsing MetaData")
	cmd.Execute()
}
