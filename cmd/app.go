package cmd

import (
	"github.com/spf13/cobra"
)

var appCmd = &cobra.Command{
	Use:     "app",
	Short:   "Various app functions.",
	Long:    "Inspect and execute various app functions..",
	Example: "app",
	Run: func(cmd *cobra.Command, args []string) {

	},
}

func init() {
	rootCmd.AddCommand(appCmd)
}
