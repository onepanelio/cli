package cmd

import (
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/onepanelio/cli/config"
	"github.com/onepanelio/cli/files"
	"github.com/onepanelio/cli/manifest"
	"github.com/onepanelio/cli/template"
	"github.com/onepanelio/cli/util"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v2"
)

const (
	manifestsFilePath = ".onepanel/manifests"
)

var (
	ConfigurationFilePath string
	ParametersFilePath    string
	Provider              string
	DNS                   string
	Dev                   bool
	EnableEFKLogging      bool
	EnableHTTPS           bool
	EnableCertManager     bool
	GPUDevicePlugins      []string
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
	Short: "Gets latests manifests and generates params.yaml file.",
	Run: func(cmd *cobra.Command, args []string) {
		if EnableCertManager && !EnableHTTPS {
			log.Printf("enable-https flag is required when enable-cert-manager is set")
			return
		}

		if EnableCertManager && DNS == "" {
			log.Printf("cert-manager-dns-provider flag is required when enable-cert-manager is set")
			return
		}

		if !EnableCertManager && DNS != "" {
			log.Printf("enable-cert-manager flag is required when cert-manager-dns-provider is set")
			return
		}

		if err := validateProvider(Provider); err != nil {
			fmt.Println(err.Error())
			return
		}

		if err := validateDNS(DNS); err != nil {
			fmt.Println(err.Error())
			return
		}

		if GPUDevicePlugins != nil {
			for _, p := range GPUDevicePlugins {
				if p != "amd" && p != "nvidia" {
					log.Printf("%v is not a valid --gpu-device-plugins value", p)
					return
				}
			}
		}

		log.Printf("Initializing...")
		configFile := ".onepanel/cli_config.yaml"
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

		if err := addCertManagerDNSProviderToManifestBuilder(DNS, bld); err != nil {
			log.Printf("[error] Adding Dns Provider: %v", err.Error())
			return
		}

		if EnableEFKLogging {
			if err := bld.AddComponent("logging"); err != nil {
				log.Printf("[error] Adding logging component: %v", err.Error())
				return
			}
		}

		if GPUDevicePlugins != nil {
			if err := bld.AddComponent("gpu-plugins"); err != nil {
				log.Printf("[error] Adding GPU plugins component: %v", err.Error())
				return
			}

			for _, p := range GPUDevicePlugins {
				bld.AddOverlayContender(p)
			}
		}

		if providerProperties[Provider].IsCloud {
			bld.AddOverlayContender("cloud")
		} else {
			bld.AddOverlayContender("local")
		}

		if EnableHTTPS {
			bld.AddOverlayContender("https")
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

		mergedParams, err := util.LoadDynamicYamlFromFile(ParametersFilePath)
		if err != nil {
			log.Printf("[error] loading params file: %v", err.Error())
			return
		}

		for _, newYaml := range bld.GetYamls() {
			mergedParams.Merge(newYaml)
		}

		if err := filterMergedParams(Provider, mergedParams); err != nil {
			log.Printf("Error filtering params: %v", err.Error())
			return
		}

		if EnableHTTPS {
			mergedParams.Put("application.insecure", false)
		} else {
			mergedParams.Put("application.insecure", true)
		}

		paramsFile, err := os.OpenFile(ParametersFilePath, os.O_RDWR, 0)
		if err != nil {
			log.Printf("Error opening parameters file: %v", err.Error())
			return
		}

		mergedParams.Sort()
		paramsString, err := mergedParams.String()
		if err != nil {
			log.Printf("[error] unable to write params to a string")
			return
		}

		if _, err := paramsFile.WriteString(paramsString); err != nil {
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

	initCmd.Flags().StringVarP(&Provider, "provider", "p", "", "Cloud provider. Valid values are: aks, gke, eks")
	initCmd.Flags().StringVarP(&DNS, "cert-manager-dns-provider", "d", "", "Provider for DNS. Valid values are: azuredns, clouddns (google), cloudflare, route53")
	initCmd.Flags().StringVarP(&ConfigurationFilePath, "config", "c", "config.yaml", "File path of the resulting config file")
	initCmd.Flags().StringVarP(&ParametersFilePath, "params", "e", "params.yaml", "File path of the resulting parameters file")
	initCmd.Flags().BoolVarP(&EnableEFKLogging, "enable-efk-logging", "", false, "Enable Elasticsearch, Fluentd and Kibana (EFK) logging")
	initCmd.Flags().BoolVarP(&EnableHTTPS, "enable-https", "", false, "Enable HTTPS scheme and redirect all requests to https://")
	initCmd.Flags().BoolVarP(&EnableCertManager, "enable-cert-manager", "", false, "Automatically create/renew TLS certs using Let's Encrypt")
	initCmd.Flags().StringSliceVarP(&GPUDevicePlugins, "gpu-device-plugins", "", nil, "Install NVIDIA and/or AMD gpu device plugins. Valid values can be comma separated and are: amd, nvidia")

	initCmd.MarkFlagRequired("provider")
}

func ValidateProvider(prov string) error {
	return validateProvider(prov)
}

func validateProvider(prov string) error {
	_, ok := providerProperties[prov]
	if !ok {
		return fmt.Errorf("Unsupported provider %v", prov)
	}

	return nil
}

func validateDNS(dns string) error {
	if dns != "route53" && dns != "" && dns != "clouddns" && dns != "azuredns" && dns != "cloudflare" {
		return fmt.Errorf("unsupported dns %v", dns)
	}

	return nil
}

func addCloudProviderToManifestBuilder(provider string, builder *manifest.Builder) error {
	builder.AddOverlayContender(provider)

	if (provider != "minikube" && provider != "microk8s") && EnableCertManager {
		if err := builder.AddComponent("cert-manager"); err != nil {
			return err
		}
	}
	if err := builder.AddComponent("storage"); err != nil {
		return err
	}
	return nil
}

func addCertManagerDNSProviderToManifestBuilder(dns string, builder *manifest.Builder) error {
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
