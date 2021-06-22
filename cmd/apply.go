package cmd

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/onepanelio/cli/util"

	opConfig "github.com/onepanelio/cli/config"
	"github.com/onepanelio/cli/files"
	"github.com/spf13/cobra"
)

// applyCmd represents the apply command
var applyCmd = &cobra.Command{
	Use:   "apply",
	Short: "Applies application YAML to your Kubernetes cluster.",
	Run: func(cmd *cobra.Command, args []string) {
		configFilePath := "config.yaml"

		fmt.Printf("Starting deployment...\n\n")

		if len(args) > 1 {
			configFilePath = args[0]
			return
		}

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
			database, err = GetDatabaseConfigurationFromCluster()
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

		existsApp, err := files.Exists(applicationKubernetesYamlFilePath)
		if err != nil {
			log.Printf("Unable to check if file '%v' exists", applicationKubernetesYamlFilePath)
			return
		}

		var applicationKubernetesFile *os.File = nil
		if !existsApp {
			applicationKubernetesFile, err = os.Create(applicationKubernetesYamlFilePath)
			if err != nil {
				log.Printf("Unable to create file: error %v", err.Error())
				return
			}
		} else {
			applicationKubernetesFile, err = os.OpenFile(applicationKubernetesYamlFilePath, os.O_RDWR|os.O_TRUNC, 0)
			if err != nil {
				log.Printf("Unable to open file: error %v", err.Error())
				return
			}
		}

		if _, err := applicationKubernetesFile.WriteString(applicationResult); err != nil {
			log.Printf("Error writing to temporary file: %v", err.Error())
			return
		}

		resApp := ""
		errResApp := ""

		resApp, errResApp, err = applyKubernetesFile(applicationKubernetesYamlFilePath)
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
				fmt.Printf("Unable to connect to cluster. Make sure you are running with \nKUBECONFIG=./kubeconfig opctl apply\nError: %v", err.Error())
				return
			}

			fmt.Printf("\nFailed: %v", err.Error())
			return
		}

		log.Printf("res: %v", resApp)
		if errResApp != "" {
			log.Printf("err: %v", errResApp)
		}

		//Once applied, verify the application is running before moving on with the rest
		//of the yaml.
		applicationRunning := false
		podName := "application-controller-manager-0"
		podNamespace := "application-system"
		podInfoRes := ""
		podInfoErrRes := ""
		var podInfoErr error
		for !applicationRunning {
			podInfoRes, podInfoErrRes, podInfoErr = getPodInfo(podName, podNamespace)
			if podInfoErr != nil {
				if !strings.Contains(podInfoErr.Error(), "not found") {
					fmt.Printf("\nFailed: %v", podInfoErr.Error())
					return
				} else {
					podInfoRes = podInfoErr.Error()
				}
			}
			if podInfoErrRes != "" {
				fmt.Printf("\n: %v", podInfoErrRes)
				return
			}
			if podInfoRes == "" {
				fmt.Printf("\nNo response from first pod check.")
				return
			}

			lines := strings.Split(podInfoRes, "\n")
			if len(lines) > 1 {
				if strings.Contains(lines[1], "Running") {
					applicationRunning = true
				}
			}
		}

		//Apply the rest of the yaml
		kustomizeTemplate := TemplateFromSimpleOverlayedComponents(config.GetOverlayComponents(overlayComponentFirst))

		result, err := GenerateKustomizeResult(kustomizeTemplate, options)
		if err != nil {
			fmt.Printf("%s\n", HumanizeKustomizeError(err))
			return
		}

		fmt.Printf(result)

		finalKubernetesYamlFilePath := filepath.Join(".onepanel", "kubernetes.yaml")

		exists, err := files.Exists(finalKubernetesYamlFilePath)
		if err != nil {
			log.Printf("Unable to check if file %v exists", finalKubernetesYamlFilePath)
			return
		}

		var finalKubernetesFile *os.File = nil
		if !exists {
			finalKubernetesFile, err = os.Create(finalKubernetesYamlFilePath)
			if err != nil {
				log.Printf("Unable to create file: error %v", err.Error())
				return
			}
		} else {
			finalKubernetesFile, err = os.OpenFile(finalKubernetesYamlFilePath, os.O_RDWR|os.O_TRUNC, 0)
			if err != nil {
				log.Printf("Unable to open file: error %v", err.Error())
				return
			}
		}

		if _, err := finalKubernetesFile.WriteString(result); err != nil {
			log.Printf("Error writing to temporary file: %v", err.Error())
			return
		}

		res := ""
		errRes := ""

		for i := 0; i < 5; i++ {
			res, errRes, err = applyKubernetesFile(finalKubernetesYamlFilePath)
			if !strings.Contains(errRes, "no matches for kind") {
				break
			}

			fmt.Printf(".")
			fmt.Printf(".")

			time.Sleep(time.Second * 3)
		}

		log.Printf("%v", res)
		if errRes != "" {
			log.Printf("%v", errRes)
		}

		yamlFile, err = util.LoadDynamicYamlFromFile(config.Spec.Params)
		if err != nil {
			fmt.Println("Error parsing configuration file.")
			return
		}
		if err != nil {
			fmt.Printf("\nDeployment failed: %v", err.Error())
		} else {
			fmt.Println("\nWaiting for deployment to complete...")
			stopChecking := false
			attempts := 0
			maxAttempts := 5
			for stopChecking == false {
				deploymentStatus, deploymentStatusErr := util.DeploymentStatus(yamlFile)
				if deploymentStatusErr != nil &&
					!strings.Contains(deploymentStatusErr.Error(), "No resources found") {
					fmt.Println(deploymentStatusErr.Error())
					stopChecking = true
				}
				if deploymentStatus {
					stopChecking = true
					fmt.Printf("\nDeployment is complete.\n\n")
				} else {
					if attempts >= maxAttempts {
						stopChecking = true
						fmt.Println("\nDeployment is still in progress. Check again with `opctl app status` in a few minutes.")
					} else {
						time.Sleep(20 * time.Second)
						fmt.Println("Waiting for deployment to complete...")
						attempts++
					}
				}
			}

			url, err := util.GetDeployedWebURL(yamlFile)
			if err != nil {
				fmt.Printf("[error] Unable to get deployed url from configuration: %v", err.Error())
				return
			}

			util.GetClusterIp(url)
		}
	},
}

func init() {
	rootCmd.AddCommand(applyCmd)
	applyCmd.Flags().BoolVarP(&Dev, "latest", "", false, "Sets conditions to allow development/latest testing.")
}

func getPodInfo(podName string, podNamespace string) (res string, errMessage string, err error) {
	var extraArgs []string
	return util.KubectlGet("pod", podName, podNamespace, extraArgs, make(map[string]interface{}))
}

func applyKubernetesFile(filePath string) (res string, errMessage string, err error) {
	return util.KubectlApply(filePath)
}
