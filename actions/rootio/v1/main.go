package main

import (
	"fmt"

	"github.com/thebsdbox/rootio/cmd"
)

func main() {

	fmt.Printf("ROOTIO - Disk Manager\n------------------------\n")
	fmt.Println("Parsing MetaData")
	// Retreive the MetaData
	cmd.Execute()
	// for fileSystem := range metadata.Storage.Filesystems {
	// 	err = fileSystemCreate(metadata.Storage.Filesystems[fileSystem])
	// 	if err != nil {
	// 		log.Error(err)
	// 	}
	// }
}
