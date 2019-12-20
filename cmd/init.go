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
	"github.com/onepanelio/cli/config"
	"github.com/onepanelio/cli/files"
	"github.com/onepanelio/cli/template"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v2"
	"log"
	"os"
)

var (
	ConfigurationFilePath string
	ParametersFilePath string
	Provider string
	Dns string
)

// initCmd represents the init command
var initCmd = &cobra.Command{
	Use:   "init <manifiests repo path>",
	Short: "Generates a sample configuration file.",
	Long: `Generates a sample configuration file and outputs it to the first argument.
If there is no argument, configuration.yaml is used.`,
	Run: func(cmd *cobra.Command, args []string) {
		if len(args) < 1 {
			fmt.Println("missing: <manifests repo path> ")
			return
		}

		manifestsRepoPath := args[0]

		if Provider == "" {
			Provider = "minikube"
		}

		if err := validateProvider(Provider); err != nil {
			fmt.Println(err.Error())
			return
		}

		if err := validateDns(Dns); err != nil {
			fmt.Println(err.Error())
			return
		}

		exists, err := files.Exists(ParametersFilePath)
		if err != nil {
			log.Printf("unable to check if %v file exists: %v", ParametersFilePath, err.Error())
			return
		}

		if !exists {
			if _, err := os.Create(ParametersFilePath); err != nil {
				log.Printf("unable to create %v file: %v", ParametersFilePath, err.Error())
				return
			}
		}

		setup := config.Config{
			ApiVersion: "opdef.apps.onepanel.io/v1alpha1",
			Kind:       "OpDef",
			Spec:       config.ConfigSpec{
				Components:    []string{},
				ManifestsRepo: manifestsRepoPath,
				Params:        ParametersFilePath,
			},
		}
		setup.SetCloudProvider(Provider)
		setup.SetDnsProvider(Dns)

		if Provider != "minikube" {
			setup.AddComponent("storage")
		}

		builder := template.NewBuilderFromConfig(setup)
		if err := builder.Build(); err != nil {
			log.Printf("err generating config. Error %v", err.Error())
			return
		}

		mergedParams, err := mergeParametersFiles(ParametersFilePath, builder.VarsArray())
		if err != nil {
			log.Printf("Error merging parameters: %v", err.Error())
			return
		}

		paramsFile, err := os.OpenFile(ParametersFilePath, os.O_RDWR, 0)
		if err != nil {
			log.Printf("Error opening parameters file: %v", err.Error())
			return
		}

		if _, err := paramsFile.WriteString(mergedParams); err != nil {
			log.Printf("Error writing merged parameters: %v", err.Error())
			return
		}

		file, err := os.Create(ConfigurationFilePath)
		if err != nil {
			log.Printf("unable to create %v file: %v", ConfigurationFilePath, err.Error())
			return
		}

		setupData, err := yaml.Marshal(setup)
		if err != nil {
			log.Printf("unable to marshal yaml data: %v", err.Error())
			return
		}

		if _, err := file.Write(setupData); err != nil {
			log.Printf("unable to write yaml data: %v", err.Error())
			return
		}

		fmt.Printf("Configuration has been created with\n")
		fmt.Printf("- Provider: %v\n", Provider)

		if Dns != "" {
			fmt.Printf("- Dns: %v\n", Dns)
		}

		fmt.Printf("- Configuration file: %v\n", ConfigurationFilePath)
		fmt.Printf("- Parameters file has been created with placeholders: %v\n", ParametersFilePath)
	},
}


func init() {
	rootCmd.AddCommand(initCmd)

	initCmd.Flags().StringVarP(&Provider, "provider", "p", "minikube", "Provider you are using. Valid values are: aws, gcp, azure, or minikube")
	initCmd.Flags().StringVarP(&Dns, "dns", "d", "", "Provider for DNS. Valid values are: aws for route53")
	initCmd.Flags().StringVarP(&ConfigurationFilePath, "config", "c", "config.yaml", "File path of the resulting config file")
	initCmd.Flags().StringVarP(&ParametersFilePath, "params", "e", "params.env", "File path of the resulting parameters file")
}

func validateProvider(prov string) error {
	if prov != "gcp" && prov != "aws" && prov != "azure" && prov != "minikube" {
		return fmt.Errorf("unsupported provider %v", prov)
	}

	return nil
}

func validateDns(dns string) error {
	if dns != "aws" && dns != "" {
		return fmt.Errorf("unsupported dns %v", dns)
	}

	return nil
}
