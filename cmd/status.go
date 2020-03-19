package cmd

import (
	"fmt"

	opConfig "github.com/onepanelio/cli/config"
	"github.com/onepanelio/cli/util"
	"github.com/spf13/cobra"
)

var statusCmd = &cobra.Command{
	Use:     "status",
	Short:   "Check deployment status.",
	Long:    "Check deployment status by checking pods statuses.",
	Example: "status",
	Run: func(cmd *cobra.Command, args []string) {
		configFilePath := "config.yaml"
		config, err := opConfig.FromFile(configFilePath)
		if err != nil {
			fmt.Printf("Unable to read configuration file: %v", err.Error())
			return
		}
		yamlFile, err := util.LoadDynamicYamlFromFile(config.Spec.Params)
		if err != nil {
			fmt.Println("Error parsing configuration file.")
			return
		}
		ready, err := util.DeploymentStatus(yamlFile)
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
