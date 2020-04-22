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

var ipCmd = &cobra.Command{
	Use:     "ip",
	Short:   "Get your cluster ip.",
	Long:    "Will give instructions for setting up cluster ip and what it will be once set.",
	Example: "opctl app ip",
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

		url, err := util.GetDeployedWebURL(yamlFile)
		if err != nil {
			fmt.Printf("[error] Unable to get deployed url from configuration: %v", err.Error())
			return
		}

		// No need to get cluster IP if local deployment
		if yamlFile.HasKey("application.local") {
			fmt.Printf("Your application is running at %v\n\n", url)
			return
		}
		util.GetClusterIp(url)
	},
}

func init() {
	rootCmd.AddCommand(appCmd)
	appCmd.AddCommand(ipCmd)
}
