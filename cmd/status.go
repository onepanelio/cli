package cmd

import (
	"fmt"

	"github.com/onepanelio/cli/util"
	"github.com/spf13/cobra"
)

var statusCmd = &cobra.Command{
	Use:     "status",
	Short:   "Check deployment status.",
	Long:    "Check deployment status by checking pods statuses.",
	Example: "status",
	Run: func(cmd *cobra.Command, args []string) {
		ready, err := util.DeploymentStatus()
		if err != nil {
			fmt.Println(err.Error())
			return
		}
		if ready {
			fmt.Println("Your deployment is ready.")
		} else {
			fmt.Println("Not all required pods are running. Your deployment is not ready.")
		}
	},
}

func init() {
	rootCmd.AddCommand(statusCmd)
}
