package cmd

import (
	"fmt"
	opConfig "github.com/onepanelio/cli/config"
	"github.com/onepanelio/cli/files"
	"github.com/spf13/cobra"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"strings"
	"time"
)

// applyCmd represents the apply command
var applyCmd = &cobra.Command{
	Use:   "deploy",
	Short: "Deploy a kubernetes yaml configuration file to your kubernetes cluster.",
	Long: `Deploys a kubernetes yaml configuration file given the 
OpDef file and parameters file, 
A sample usage is:

op-cli apply config.yaml params.env
`,
	Run: func(cmd *cobra.Command, args []string) {
		configFilePath := "config.yaml"

		if len(args) > 1 {
			configFilePath = args[0]
			return
		}

		config, err := opConfig.FromFile(configFilePath)
		if err != nil {
			fmt.Printf("Unable to read configuration file: %v", err.Error())
			return
		}

		kustomizeTemplate := TemplateFromSimpleOverlayedComponents(config.GetOverlayComponents())

		result, err := GenerateKustomizeResult(*config, kustomizeTemplate)
		if err != nil {
			log.Printf("Error generating result %v", err.Error())
			return
		}

		finalKubernetesYamlFilePath := ".kubernetes.yaml"

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

		fmt.Printf("Deploying...\n")

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

		if err != nil {
			fmt.Printf("\nFailed: %v", err.Error())
		} else {
			fmt.Printf("\nFinished applying\n")
		}
	},
}

func init() {
	rootCmd.AddCommand(applyCmd)

	// Here you will define your flags and configuration settings.

	// Cobra supports Persistent Flags which will work for this command
	// and all subcommands, e.g.:
	// applyCmd.PersistentFlags().String("foo", "", "A help for foo")

	// Cobra supports local flags which will only run when this command
	// is called directly, e.g.:
	// applyCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
}

func applyKubernetesFile(filePath string) (res string, errMessage string, err error) {
	cmd := exec.Command("kubectl", "apply", "-f", filePath, "--validate=false")
	stdOut, err := cmd.StdoutPipe()
	if err != nil {
		return "", "", err
	}

	stdErr, err := cmd.StderrPipe()
	if err != nil {
		return "", "", err
	}

	if err := cmd.Start(); err != nil {
		return "", "", err
	}

	result, err := ioutil.ReadAll(stdOut)
	if err != nil {
		return "", "", err
	}

	errRes, err := ioutil.ReadAll(stdErr)
	if err != nil {
		return "", "", err
	}

	if err := cmd.Wait(); err != nil {
		return string(result), string(errRes), err
	}

	return string(result), string(errRes), nil
}
