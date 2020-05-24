package cmd

import (
	"fmt"

	"github.com/onepanelio/cli/config"
	"github.com/spf13/cobra"
)

var versionCmd = &cobra.Command{
	Use:     "version",
	Short:   "Returns the current version of the CLI",
	Long:    "Returns the current version of the CLI",
	Example: "version",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("CLI version: %v\nManifest version: %v\n", config.VersionTag, config.VersionTag)
	},
}

func init() {
	rootCmd.AddCommand(versionCmd)
}
