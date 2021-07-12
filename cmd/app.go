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
		k8sClient, err := util.NewKubernetesClient()
		if err != nil {
			fmt.Printf("Unable to create kubernetes client: error %v", err.Error())
			return
		}

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

		ready, err := util.NamespacesExist(k8sClient, util.NamespacesToCheck(yamlFile)...)
		if err != nil {
			yamlFile, yamlErr := util.LoadDynamicYamlFromFile(config.Spec.Params)
			if yamlErr != nil {
				fmt.Printf("Error reading file '%v' %v", config.Spec.Params, yamlErr.Error())
				return
			}

			flatMap := yamlFile.FlattenToKeyValue(util.AppendDotFlatMapKeyFormatter)
			provider, providerErr := util.GetYamlStringValue(flatMap, "application.provider")
			if providerErr != nil {
				fmt.Printf("Unable to read application.provider from params.yaml %v", providerErr.Error())
				return
			}
			if provider == nil {
				fmt.Printf("application.provider is not set in params.yaml")
				return
			}

			if *provider == "microk8s" {
				fmt.Printf("Unable to connect to cluster. Make sure you are running with \nKUBECONFIG=./kubeconfig opctl app status\nError: %v", err.Error())
				return
			}

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

		util.PrintClusterNetworkInformation(url)
	},
}

func init() {
	rootCmd.AddCommand(appCmd)
	appCmd.AddCommand(statusCmd)
}
