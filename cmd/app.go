package cmd

import (
	"github.com/spf13/cobra"
)

var appCmd = &cobra.Command{
	Use:     "app",
	Short:   "Various app functions.",
	Long:    "Inspect and execute various app functions..",
	Example: "app",
	Run:     func(cmd *cobra.Command, args []string) {},
}

var ipCmd = &cobra.Command{
	Use:     "ip",
	Short:   "Get your cluster ip.",
	Long:    "Will give instructions for setting up cluster ip and what it will be once set.",
	Example: "opctl app ip",
	Run: func(cmd *cobra.Command, args []string) {
		print("opctl app ip")
	},
}

func init() {
	rootCmd.AddCommand(appCmd)
	appCmd.AddCommand(ipCmd)
}
