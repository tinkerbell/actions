package cmd

import (
	"github.com/spf13/cobra"
	"go.uber.org/zap"
)

var log *zap.Logger
var rootCmd = &cobra.Command{
	Use:  "hub",
	Long: ``,
}

// Execute starts the command line interface.
func Execute(zap *zap.Logger) {
	// Make the logger available in subcommands.
	log = zap

	if err := rootCmd.Execute(); err != nil {
		log.Fatal(err.Error())
	}
}
