package cmd

import (
	"github.com/spf13/cobra"
	"go.uber.org/zap"
)

var (
	logger  *zap.Logger
	rootCmd = &cobra.Command{
		Use:  "hub",
		Long: ``,
	}
)

// Execute starts the command line interface.
func Execute(l *zap.Logger) {
	// Make the logger available in subcommands.
	logger = l

	if err := rootCmd.Execute(); err != nil {
		logger.Fatal(err.Error())
	}
}
