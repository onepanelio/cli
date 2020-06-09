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

		overlayComponentFirst := filepath.Join("common/application/base")
		baseOverlayComponent := config.GetOverlayComponent(overlayComponentFirst)
		applicationBaseKustomizeTemplate := TemplateFromSimpleOverlayedComponents(baseOverlayComponent)
		applicationResult, err := GenerateKustomizeResult(*config, applicationBaseKustomizeTemplate)
		if err != nil {
			log.Printf("Error generating result %v", err.Error())
			return
		}

		applicationKubernetesYamlFilePath := filepath.Join(".onepanel/application.kubernetes.yaml")

		existsApp, err := files.Exists(applicationKubernetesYamlFilePath)
		if err != nil {
			log.Printf("Unable to check if file %v exists", applicationKubernetesYamlFilePath)
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

		log.Printf("%v", resApp)
		if errResApp != "" {
			log.Printf("%v", errResApp)
		}

		if err != nil {
			fmt.Printf("\nFailed: %v", err.Error())
			return
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

		result, err := GenerateKustomizeResult(*config, kustomizeTemplate)
		if err != nil {
			log.Printf("Error generating result %v", err.Error())
			return
		}

		finalKubernetesYamlFilePath := filepath.Join(".onepanel/kubernetes.yaml")

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

		yamlFile, err := util.LoadDynamicYamlFromFile(config.Spec.Params)
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

			// No need to get cluster IP if local deployment
			if yamlFile.HasKey("application.local") {
				fmt.Printf("Your application is running at %v\n\n", url)
				return
			}
			util.GetClusterIp(url)
		}
	},
}

func init() {
	rootCmd.AddCommand(applyCmd)
	applyCmd.Flags().BoolVarP(&Dev, "dev", "", false, "Sets conditions to allow development testing.")
}

func getPodInfo(podName string, podNamespace string) (res string, errMessage string, err error) {
	var extraArgs []string
	return util.KubectlGet("pod", podName, podNamespace, extraArgs, make(map[string]interface{}))
}

func applyKubernetesFile(filePath string) (res string, errMessage string, err error) {
	return util.KubectlApply(filePath)
}