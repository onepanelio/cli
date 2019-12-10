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
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v2"
	"log"
	"os"
)

// initCmd represents the init command
var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Generates a sample configuration file.",
	Long: `Generates a sample configuration file and outputs it to the first argument.
If there is no argument, configuration.yaml is used.`,
	Run: func(cmd *cobra.Command, args []string) {
		paramsPath := "params.env"
		configurationFilePath := "configuration.yaml"

		if len(args) > 0 {
			configurationFilePath = args[0]
		}

		exists, err := files.Exists(paramsPath)
		if err != nil {
			log.Printf("unable to check if params.env file exists: %v", err.Error())
			return
		}

		if !exists {
			if _, err := os.Create(paramsPath); err != nil {
				log.Printf("unable to create params.env file: %v", err.Error())
				return;
			}
		}

		setup := config.Config{
			ApiVersion: "opdef.apps.onepanel.io/v1alpha1",
			Kind:       "OpDef",
			Spec:       config.ConfigSpec{
				ManifestsRepo: "TODO",
				Params:        "params.env",
				Components:    []string{"isto", "argo", "storage"},
				Overlays:      []string{"storage/overlays/gcp", "common/cert-manager/overlays/aws"},
			},
		}

		file, err := os.Create(configurationFilePath)
		if err != nil {
			log.Printf("unable to create configuration.yaml file: %v", err.Error())
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

		fmt.Printf("An example file has been generated: %v\n", configurationFilePath)
	},
}


func init() {
	rootCmd.AddCommand(initCmd)

	// Here you will define your flags and configuration settings.

	// Cobra supports Persistent Flags which will work for this command
	// and all subcommands, e.g.:
	// initCmd.PersistentFlags().String("foo", "", "A help for foo")

	// Cobra supports local flags which will only run when this command
	// is called directly, e.g.:
	// initCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
}
