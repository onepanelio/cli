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
	"github.com/onepanelio/cli/manifest"
	"github.com/onepanelio/cli/template"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v2"
	"log"
	"os"
	"strings"
)

const (
	manifestsFilePath = ".manifests"
)

var (
	ConfigurationFilePath string
	ParametersFilePath string
	Provider string
	Dns string
	LoggingComponent bool
)

// initCmd represents the init command
var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Generates a sample configuration file.",
	Long: `Generates a sample configuration file and outputs it to the first argument.
If there is no argument, configuration.yaml is used.`,
	Run: func(cmd *cobra.Command, args []string) {
		configFile := ".cli_config.yaml"
		exists, err := files.Exists(configFile)
		if err != nil {
			log.Printf("[error] checking for config file %v", configFile)
			return
		}

		if !exists {
			if err := manifest.CreateGithubSourceConfigFile(configFile); err != nil {
				log.Printf("[error] creating default source config: %v", err.Error())
				return
			}
		}

		source, err := manifest.LoadManifestSourceFromFileConfig(configFile);
		if err != nil {
			log.Printf("[error] loading manifest source: %v", err.Error())
			return
		}

		if err := source.MoveToDirectory(manifestsFilePath); err != nil {
			log.Printf("[error] %v", err.Error())
			return
		}

		manifestsRepoPath, err := source.GetManifestPath()
		if err != nil {
			log.Printf("[error] %v", err.Error())
			return
		}

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

		exists, err = files.Exists(ParametersFilePath)
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

		loadedManifest, err := manifest.LoadManifest(manifestsRepoPath)
		if err != nil {
			log.Printf("[error] LoadManifest %v", err.Error())
		}

		skipList := make([]string, 0)
		if Provider == "minikube" {
			skipList = append(skipList, "common" + string(os.PathSeparator) + "istio")
		}

		bld := manifest.CreateBuilder(loadedManifest)
		if err := bld.AddCommonComponents(skipList...); err != nil {
			log.Printf("[error] AddCommonComponents %v", err.Error())
			return
		}

		if err := addCloudProviderToManifestBuilder(Provider, bld); err != nil {
			log.Printf("[error] Adding Cloud Provider: %v", err.Error())
			return
		}

		if err := addDnsProviderToManifestBuilder(Dns, bld); err != nil {
			log.Printf("[error] Adding Dns Provider: %v", err.Error())
			return
		}

		if LoggingComponent {
			if err := bld.AddComponent("logging"); err != nil {
				log.Printf("[error] Adding logging component: %v", err.Error())
				return
			}
		}

		if err := bld.Build(); err != nil {
			log.Printf("[error] building components and overlays: %v", err.Error())
			return
		}

		for _, overlayComponent := range bld.GetOverlayComponents() {
			setup.AddComponent(overlayComponent.Component().PathWithBase())
			for _, overlay := range overlayComponent.Overlays() {
				setup.AddOverlay(overlay.Path())
			}
		}

		builder := template.NewBuilderFromConfig(setup)
		if err := builder.Build(); err != nil {
			log.Printf("err generating config. Error %v", err.Error())
			return
		}

		mergedParams, err := files.MergeParametersFiles(ParametersFilePath, bld.GetVarsArray())
		if err != nil {
			log.Printf("Error merging parameters: %v", err.Error())
			return
		}

		paramsFile, err := os.OpenFile(ParametersFilePath, os.O_RDWR, 0)
		if err != nil {
			log.Printf("Error opening parameters file: %v", err.Error())
			return
		}

		if err := mergedParams.WriteToFile(paramsFile); err != nil {
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
	initCmd.Flags().StringVarP(&ParametersFilePath, "params", "e", "params.yaml", "File path of the resulting parameters file")
	initCmd.Flags().BoolVarP(&LoggingComponent, "logging", "l", false, "If set, adds a logging component")
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

func addCloudProviderToManifestBuilder(provider string, builder *manifest.Builder) error {
	builder.AddOverlayContender(provider)

	if provider == "minikube" {
		return nil
	}

	if err := builder.AddComponent("cert-manager"); err != nil {
		return err
	}

	if err := builder.AddComponent("storage"); err != nil {
		return err
	}

	return nil
}

func addDnsProviderToManifestBuilder(dns string, builder *manifest.Builder) error {
	if dns == "" {
		return nil
	}

	builder.AddOverlayContender(dns)

	overlay := strings.Join([]string{"cert-manager","overlays",dns},string(os.PathSeparator))
	return builder.AddOverlay(overlay)
}
