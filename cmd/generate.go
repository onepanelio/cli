/*
Copyright © 2019 NAME HERE <EMAIL ADDRESS>

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/
package cmd

import (
	"fmt"
	opConfig "github.com/onepanelio/cli/config"
	"github.com/onepanelio/cli/files"
	"github.com/onepanelio/cli/template"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v2"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// generateCmd represents the generate command
var generateCmd = &cobra.Command{
	Use:   "generate",
	Short: "Generates a kubernetes yaml configuration file.",
	Long: `Generates a kubernetes yaml configuration file given the 
OpDef file and parameters file, where you can customize components and overlays.

A sample usage is:

op-cli generate config.yaml params.env
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

		builder := template.NewBuilderFromConfig(*config)
		if err := builder.Build(); err != nil {
			log.Printf("err generating config. Error %v", err.Error())
			return
		}

		kustomizeTemplate := builder.Template()

		result, err := GenerateKustomizeResult(*config, kustomizeTemplate)
		if err != nil {
			log.Printf("Error generating result %v", err.Error())
			return
		}

		fmt.Printf("%v", result)
	},
}

func init() {
	rootCmd.AddCommand(generateCmd)

	// Here you will define your flags and configuration settings.
	//generateCmd.Flags().StringVarP(&configPath, "configPath", "c", "minikube", "Cloud provider to use. Example: AWS|GCP|Azure|Minikube")
	// Cobra supports Persistent Flags which will work for this command
	// and all subcommands, e.g.:
	// generateCmd.PersistentFlags().String("foo", "", "A help for foo")

	// Cobra supports local flags which will only run when this command
	// is called directly, e.g.:
	// generateCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
}

// Given the path to the manifests, and a kustomize config, creates the final kustomization file.
// It does this by copying the manifests into a temporary directory, inserting the kustomize template
// and running the kustomize command
func GenerateKustomizeResult(config opConfig.Config, kustomizeTemplate template.Kustomize) (string, error) {
	manifestPath := config.Spec.ManifestsRepo
	localManifestsCopyPath := ".manifest"

	exists, err := files.Exists(localManifestsCopyPath)
	if err != nil {
		return "", err
	}

	if exists {
		if err := os.RemoveAll(localManifestsCopyPath); err != nil {
			return "", err
		}
	}

	if err := files.CopyDir(manifestPath, localManifestsCopyPath); err != nil {
		return "", err
	}

	localKustomizePath := filepath.Join(localManifestsCopyPath, "kustomization.yaml")
	if _, err := files.DeleteIfExists(localKustomizePath); err != nil {
		return "", err
	}

	newFile, err := os.Create(localKustomizePath)
	if err != nil {
		return "", err
	}

	kustomizeYaml, err := yaml.Marshal(kustomizeTemplate)
	if err != nil {
		log.Printf("Error yaml. Error %v", err.Error())
		return "", err
	}

	_, err = newFile.Write(kustomizeYaml)
	if err != nil {
		return "", err
	}

	paramsPath := filepath.Join(localManifestsCopyPath, "vars", "params.env")
	if _, err := files.DeleteIfExists(paramsPath); err != nil {
		return "", err
	}

	if err := files.CopyFile(config.Spec.Params, paramsPath); err != nil {
		return "", err
	}

	cmd := exec.Command("kustomize", "build", ".manifest",  "--load_restrictor",  "none")
	stdOut, err := cmd.StdoutPipe()
	if err != nil {
		return "", err
	}

	stdErr, err := cmd.StderrPipe()
	if err != nil {
		return "", err
	}

	if err := cmd.Start(); err != nil {
		return "", err
	}

	result, err := ioutil.ReadAll(stdOut)
	if err != nil {
		return "", err
	}

	errRes, err := ioutil.ReadAll(stdErr)
	if err != nil {
		log.Printf("Errors:\n%v", string(errRes))
		return "", err
	}

	if err := cmd.Wait(); err != nil {
		return "", err
	}

	return string(result), nil
}

// Given the .env file at path (assumed to exist)
// read through, and add any variables that are not in newVars with a value of TODO
// e.g.
// email=TODO
func mergeParametersFiles(path string, newVars []string) (result string, err error) {
	mappedVars := make(map[string]bool)
	for i := range newVars {
		varName := newVars[i]
		mappedVars[varName] = true
	}

	fileData, err := ioutil.ReadFile(path)
	if err != nil {
		return
	}

	fileString := string(fileData)
	result = ""

	fileLines := strings.Split(fileString, "\n")
	for i := range fileLines {
		fileLine := fileLines[i]

		envVarParts := strings.Split(fileLine, "=")
		if len(envVarParts) > 1 {
			varName := envVarParts[0]
			if _, ok := mappedVars[varName]; ok {
				delete(mappedVars, varName)
			}
		}

		if i == (len(fileLines) - 1) {
			result += fileLine
		} else {
			result += fileLine + "\n"
		}
	}

	for key := range mappedVars {
		result += fmt.Sprintf("\n%v=%v", key, "TODO")
	}

	return
}