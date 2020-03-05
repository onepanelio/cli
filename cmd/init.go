package cmd

import (
	"fmt"
	"github.com/onepanelio/cli/config"
	"github.com/onepanelio/cli/files"
	"github.com/onepanelio/cli/manifest"
	"github.com/onepanelio/cli/template"
	"github.com/onepanelio/cli/util"
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
	ParametersFilePath    string
	Provider              string
	DNS                   string
	LoggingComponent      bool
)

type ProviderProperties struct {
	IsCloud bool
}

var providerProperties = map[string]ProviderProperties{
	"minikube": {
		IsCloud: false,
	},
	"microk8s": {
		IsCloud: false,
	},
	"gke": {
		IsCloud: true,
	},
	"eks": {
		IsCloud: true,
	},
	"aks": {
		IsCloud: true,
	},
}

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

		source, err := manifest.LoadManifestSourceFromFileConfig(configFile)
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

		if err := validateDNS(DNS); err != nil {
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
			Spec: config.ConfigSpec{
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
		if !providerProperties[Provider].IsCloud {
			skipList = append(skipList, "common"+string(os.PathSeparator)+"istio")
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

		if err := addDNSProviderToManifestBuilder(DNS, bld); err != nil {
			log.Printf("[error] Adding Dns Provider: %v", err.Error())
			return
		}

		if LoggingComponent {
			if err := bld.AddComponent("logging"); err != nil {
				log.Printf("[error] Adding logging component: %v", err.Error())
				return
			}
		}

		if providerProperties[Provider].IsCloud {
			bld.AddOverlayContender("cloud")
		} else {
			bld.AddOverlayContender("local")
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

		if err := filterMergedParams(Provider, mergedParams); err != nil {
			log.Printf("Error filtering params: %v", err.Error())
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

		if DNS != "" {
			fmt.Printf("- Dns: %v\n", DNS)
		}

		fmt.Printf("- Configuration file: %v\n", ConfigurationFilePath)
		fmt.Printf("- Parameters file has been created with placeholders: %v\n", ParametersFilePath)
	},
}

func init() {
	rootCmd.AddCommand(initCmd)

	initCmd.Flags().StringVarP(&Provider, "provider", "p", "minikube", "Provider you are using. Valid values are: aws, gcp, azure, or minikube")
	initCmd.Flags().StringVarP(&DNS, "dns", "d", "", "Provider for DNS. Valid values are: aws for route53")
	initCmd.Flags().StringVarP(&ConfigurationFilePath, "config", "c", "config.yaml", "File path of the resulting config file")
	initCmd.Flags().StringVarP(&ParametersFilePath, "params", "e", "params.yaml", "File path of the resulting parameters file")
	initCmd.Flags().BoolVarP(&LoggingComponent, "logging", "l", false, "If set, adds a logging component")
}

func validateProvider(prov string) error {
	_, ok := providerProperties[prov]
	if !ok {
		return fmt.Errorf("unsupported provider %v", prov)
	}

	return nil
}

func validateDNS(dns string) error {
	if dns != "aws" && dns != "" {
		return fmt.Errorf("unsupported dns %v", dns)
	}

	return nil
}

func addCloudProviderToManifestBuilder(provider string, builder *manifest.Builder) error {
	builder.AddOverlayContender(provider)

	if provider != "minikube" && provider != "microk8s" {
		if err := builder.AddComponent("cert-manager"); err != nil {
			return err
		}
	}
	if err := builder.AddComponent("storage"); err != nil {
		return err
	}
	return nil
}

func addDNSProviderToManifestBuilder(dns string, builder *manifest.Builder) error {
	if dns == "" {
		return nil
	}

	builder.AddOverlayContender(dns)

	overlay := strings.Join([]string{"cert-manager", "overlays", dns}, string(os.PathSeparator))
	return builder.AddOverlay(overlay)
}

func filterMergedParams(provider string, mergedParams *util.DynamicYaml) error {
	keyToDelete := "application.local"
	if !providerProperties[provider].IsCloud {
		keyToDelete = "application.cloud"
	}

	if err := mergedParams.DeleteByString(keyToDelete, "."); err != nil {
		return err
	}

	return nil
}
