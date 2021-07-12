package cmd

import (
	"fmt"
	"io/ioutil"
	"log"
	"path/filepath"
	"time"

	"github.com/onepanelio/cli/util"

	opConfig "github.com/onepanelio/cli/config"
	"github.com/spf13/cobra"
)

// applyCmd represents the apply command
var applyCmd = &cobra.Command{
	Use:   "apply",
	Short: "Applies application YAML to your Kubernetes cluster.",
	Run: func(cmd *cobra.Command, args []string) {
		configFilePath := "config.yaml"
		if len(args) > 1 {
			configFilePath = args[0]
		}

		k8sClient, err := util.NewKubernetesClient()
		if err != nil {
			fmt.Printf("Unable to create kubernetes client: error %v", err.Error())
			return
		}

		fmt.Printf("Starting deployment...\n\n")

		config, err := opConfig.FromFile(configFilePath)
		if err != nil {
			fmt.Printf("Unable to read configuration file: %v", err.Error())
			return
		}

		yamlFile, err := util.LoadDynamicYamlFromFile(config.Spec.Params)
		if err != nil {
			fmt.Printf("Unable to read params.yaml: %v", err.Error())
			return
		}

		var database *opConfig.Database = nil
		if !yamlFile.HasKey("database") {
			database, err = GetDatabaseConfigurationFromCluster(k8sClient)
			if err != nil {
				fmt.Printf("Unable to connect to cluster to check information: %v", err.Error())
				return
			}
		}

		options := &GenerateKustomizeResultOptions{
			Database: database,
			Config:   config,
		}

		overlayComponentFirst := filepath.Join("common", "application", "base")
		baseOverlayComponent := config.GetOverlayComponent(overlayComponentFirst)
		applicationBaseKustomizeTemplate := TemplateFromSimpleOverlayedComponents(baseOverlayComponent)
		applicationResult, err := GenerateKustomizeResult(applicationBaseKustomizeTemplate, options)
		if err != nil {
			fmt.Printf("%s\n", HumanizeKustomizeError(err))
			return
		}

		applicationKubernetesYamlFilePath := filepath.Join(".onepanel", "application.kubernetes.yaml")
		if err := ioutil.WriteFile(applicationKubernetesYamlFilePath, []byte(applicationResult), 0644); err != nil {
			log.Printf("Error writing to temporary file: %v", err.Error())
			return
		}

		if err := applyKubernetesFile(applicationKubernetesYamlFilePath); err != nil {
			provider := yamlFile.GetValue("application.provider").Value
			if provider == "microk8s" {
				fmt.Printf("Unable to connect to cluster. Make sure you are running with \nKUBECONFIG=./kubeconfig opctl apply\nError: %v", err.Error())
				return
			}

			fmt.Printf("\nFailed: %v", err.Error())
			return
		}

		for i := 0; i < 5; i++ {
			applicationRunning, err := util.IsApplicationControllerManagerRunning(k8sClient)
			if err != nil {
				fmt.Printf("Error checking if application is running: error %v", err.Error())
				return
			}

			if applicationRunning {
				break
			}

			time.Sleep(1 * time.Second)
		}

		//Apply the rest of the yaml
		kustomizeTemplate := TemplateFromSimpleOverlayedComponents(config.GetOverlayComponents(overlayComponentFirst))

		result, err := GenerateKustomizeResult(kustomizeTemplate, options)
		if err != nil {
			fmt.Printf("%s\n", HumanizeKustomizeError(err))
			return
		}

		finalKubernetesYamlFilePath := filepath.Join(".onepanel", "kubernetes.yaml")
		if err := ioutil.WriteFile(finalKubernetesYamlFilePath, []byte(result), 0644); err != nil {
			log.Printf("Error writing to temporary file: %v", err.Error())
			return
		}

		for i := 0; i < 5; i++ {
			err = applyKubernetesFile(finalKubernetesYamlFilePath)
			if err == nil {
				break
			}

			time.Sleep(time.Second * 5)
		}

		if config.Spec.HasLikeComponent("kfserving") {
			defaultNamespace := yamlFile.GetValue("application.defaultNamespace").Value
			filePath := filepath.Join(config.Spec.ManifestsRepo, "kfserving", "patch", "serviceaccount.yaml")

			if err := util.KubectlPatch(defaultNamespace, "serviceaccount/default", filePath); err != nil {
				fmt.Printf(err.Error())
				return
			}
		}

		attempts := 0
		maxAttempts := 5
		for attempts < maxAttempts {
			fmt.Println("\nWaiting for deployment to complete...")
			deploymentStatus, err := util.NamespacesExist(k8sClient, util.NamespacesToCheck(yamlFile)...)
			if err != nil {
				fmt.Printf("Unable to check if namespaces exist in cluster. Error %v", err.Error())
				return
			}

			if deploymentStatus {
				fmt.Printf("\nDeployment is complete.\n\n")
				break
			}

			if attempts >= maxAttempts {
				fmt.Println("\nDeployment is still in progress. Check again with `opctl app status` in a few minutes.")
				break
			}

			time.Sleep(20 * time.Second)
			attempts++
		}

		url, err := util.GetDeployedWebURL(yamlFile)
		if err != nil {
			fmt.Printf("[error] Unable to get deployed url from configuration: %v", err.Error())
			return
		}

		util.PrintClusterNetworkInformation(url)
	},
}

func init() {
	rootCmd.AddCommand(applyCmd)
	applyCmd.Flags().BoolVarP(&Dev, "latest", "", false, "Sets conditions to allow development/latest testing.")
}

func applyKubernetesFile(filePath string) (err error) {
	return util.KubectlApply(filePath)
}
