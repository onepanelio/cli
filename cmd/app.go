package cmd

import (
	"fmt"
	opConfig "github.com/onepanelio/cli/config"
	"github.com/onepanelio/cli/util"
	"github.com/spf13/cobra"
)

var appCmd = &cobra.Command{
	Use:     "app",
	Short:   "Various app functions.",
	Long:    "Inspect and execute various app functions..",
	Example: "app",
	Run:     func(cmd *cobra.Command, args []string) {},
}

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
			fmt.Println("Your deployment is NOT ready; not all Pods are running. To view all Pods:")
			fmt.Println("$ kubectl get pods -A")
		}

		// Get cluster deployment URL
		url, err := util.GetDeployedWebURL(yamlFile)
		if err != nil {
			fmt.Printf("[error] Unable to get deployed url from configuration: %v", err.Error())
			return
		}

		util.GetClusterIp(url)
	},
}

func init() {
	rootCmd.AddCommand(appCmd)
	appCmd.AddCommand(statusCmd)
}
