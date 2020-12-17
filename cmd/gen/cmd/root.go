package cmd

import (
	"github.com/spf13/cobra"
	"go.uber.org/zap"
)

var rootCmd = &cobra.Command{
	Use:  "gen",
	Long: ``,
}

func Execute(log *zap.Logger) {
	if err := rootCmd.Execute(); err != nil {
		log.Fatal(err.Error())
	}
}
