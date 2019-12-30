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
	"github.com/onepanelio/cli/github"
	"github.com/onepanelio/cli/manifest"
	"github.com/onepanelio/cli/template"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v2"
	"log"
	"os"
)

const (
	manifestsFilePath = ".manifests"
	manifestsTargetDirectory = "manifests-feature-testing-restructure"
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
		manifestExists, err := files.Exists(manifestsFilePath)
		if err != nil {
			log.Printf("[error] Unable to check if manifests cached directory exists %v", err.Error())
			return
		}

		if !manifestExists {
			if err := downloadManifestFiles(); err != nil {
				log.Printf("[error] downloading manifest files from github. Error %v", err.Error())
				return
			}
		}

		manifestsRepoPath := manifestsFilePath + string(os.PathSeparator) + manifestsTargetDirectory

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

		loadedManifest, err := manifest.LoadManifest(manifestsRepoPath)
		if err != nil {
			log.Printf("[error] LoadManifest %v", err.Error())
		}

		bld := manifest.CreateBuilder(loadedManifest)
		if err := bld.AddCommonComponents(); err != nil {
			log.Printf("[error] AddCommonComponents %v", err.Error())
			return
		}

		if Provider != "minikube" {
			if err := bld.AddComponent("storage"); err != nil {
				log.Printf("[error] Adding storage component: %v", err.Error())
				return
			}
		}

		if err := addCloudProviderToManifestBuilder(Provider, bld); err != nil {
			log.Printf("[error] Adding Cloud Provider: %v", err.Error())
			return
		}

		if err := addDnsProviderToManifestBuilder(Dns, bld); err != nil {
			log.Printf("[error] Adding Dns Provider: %v", err.Error())
			return
		}

		// TODO - what about minikube overlay? What about any kind of overlay?
		if LoggingComponent {
			if err := bld.AddComponent("logging"); err != nil {
				log.Printf("[error] Adding logging component: %v", err.Error())
				return
			}

			if Provider == "minikube" {
				if err := bld.AddOverlay("logging/overlays/minikube"); err != nil {
					log.Printf("[error] Adding logging minikube overlay: %v", err.Error())
					return
				}
			}
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
	if provider == "minikube" {
		return nil
	}

	if err := builder.AddComponent("cert-manager"); err != nil {
		return err
	}

	if err := builder.AddOverlay("storage/overlays/" + provider); err != nil {
		return err
	}

	return nil
}

func addDnsProviderToManifestBuilder(dns string, builder *manifest.Builder) error {
	if dns == "" {
		return nil
	}

	return builder.AddOverlay("cert-manager/overlays/" + dns)
}

func downloadManifestFiles() error {
	downloader := github.Github{}

	tempManifestsPath := ".temp_manifests"

	defer func () {
		_, err := files.DeleteIfExists(tempManifestsPath);
		if err != nil {
			log.Printf("[error] Deleting %v: %v", tempManifestsPath, err.Error())
		}
	}()

	if err := downloader.DownloadManifests(tempManifestsPath); err != nil {
		return err
	}

	_, err := files.Unzip(tempManifestsPath, manifestsFilePath)

	return err
}