package cmd

import "github.com/spf13/cobra"

// restartFlags groups the flags to direct server restarts
var restartFlags struct {
	hard bool // whether to do a hard reset instead of an OS-level reboot
}

func init() {
	// The restart command is like the off command, it wraps both reset and reboot
	var restart = &cobra.Command{
		Use:   "restart  [group|server [group|server]...]",
		Short: "Reboot or reset server(s)",
		Long:  "Reboot (OS-level) or reset (if --hard) a server",
		Run: func(cmd *cobra.Command, args []string) {
			if restartFlags.hard {
				serverCmd("reset", client.ResetServer, args)
			} else {
				serverCmd("reboot", client.RebootServer, args)
			}
		},
	}
	restart.Flags().BoolVar(&restartFlags.hard, "hard", false, "Whether to use a hard reset instead of an OS-level reboot")

	Root.AddCommand(restart)

	Root.AddCommand(&cobra.Command{
		Use:    "reset  [group|server [group|server]...]",
		Hidden: true,
		Short:  "Reset server(s)",
		Long:   "Performs hard/forced power-cycle, like pressing the physical 'reset' button",
		Run: func(cmd *cobra.Command, args []string) {
			serverCmd("reset", client.ResetServer, args)
		}})

	Root.AddCommand(&cobra.Command{
		Use:    "reboot  [group|server [group|server]...]",
		Hidden: true,
		Short:  "Reboot server(s)",
		Long:   "Soft (OS-level) reboot",
		Run: func(cmd *cobra.Command, args []string) {
			serverCmd("reboot", client.RebootServer, args)
		}})

}
