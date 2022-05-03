package main

import (
	"github.com/tinkerbell/actions/cmd/hub/cmd"
	"go.uber.org/zap"
)

func main() {
	logger, _ := zap.NewProduction()
	cmd.Execute(logger)
	_ = logger.Sync() // flushes buffer, if any
}
