package cmd

import (
	"fmt"
	"github.com/spf13/cobra"
)

var versionCmd = &cobra.Command{
	Use:                        "version",
	Short:                      "Returns the current version of the CLI",
	Long:                       "Returns the current version of the CLI",
	Example:                    "version",
	Run: func(cmd *cobra.Command, args []string) {
		version := "1.0.0-beta.1"
		fmt.Printf("CLI version: " + version + "\n")
	},
}

func init()  {
	rootCmd.AddCommand(versionCmd)
}