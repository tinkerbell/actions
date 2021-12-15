package main

import (
	"fmt"
	"os"

	"github.com/tinkerbell/actions/cmd/hub/cmd"
	"go.uber.org/zap"
)

func main() {
	logger, _ := zap.NewProduction()
	defer func() {
		// flushes buffer, if any
		if err := logger.Sync(); err != nil {
			fmt.Fprint(os.Stderr, "error flushing logger: ", err)
		}
	}()
	cmd.Execute(logger)
}
