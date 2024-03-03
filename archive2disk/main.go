package main

import (
	"fmt"
	"os"

	"github.com/tinkerbell/actions/archive2disk/app"
)

const mountAction = "/mountAction"

func main() {
	if err := app.Archive2Disk(os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, err)
	}
}
