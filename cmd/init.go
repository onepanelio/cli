package cmd

import (
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
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
	manifestsFilePath             = ".onepanel/manifests"
	artifactRepositoryProviderS3  = "s3"
	artifactRepositoryProviderGcs = "gcs"
	artifactRepositoryProviderAbs = "abs"
)

var (
	ConfigurationFilePath      string
	ParametersFilePath         string
	Provider                   string
	DNS                        string
	ArtifactRepositoryProvider string
	Dev                        bool
	EnableEFKLogging           bool
	EnableHTTPS                bool
	EnableCertManager          bool
	EnableMetalLb              bool
	DisableServing             bool
	Database                   bool
	GPUDevicePlugins           []string
	Services                   []string
)

// ProviderProperties are data associated with various providers, like microk8s vs eks
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
	Short: "Gets latest manifests and generates params.yaml file.",
	Run: func(cmd *cobra.Command, args []string) {
		if err := validateInput(); err != nil {
			log.Println(err.Error())
			return
		}

		log.Printf("Initializing...")
		configFile := filepath.Join(".onepanel", "cli_config.yaml")
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

		// When updating cli versions, the cli_config.yaml may already exist.
		// Check if we need to generate a new cli_config.yaml, to match the cli version.
		tag := config.ManifestsRepositoryTag
		if source.GetSourceType() == manifest.SourceGithub {
			if source.GetTag() != "" {
				if tag != source.GetTag() {
					if err := manifest.CreateGithubSourceConfigFile(configFile); err != nil {
						log.Printf("[error] creating default source config: %v", err.Error())
						return
					}
					source, err = manifest.LoadManifestSourceFromFileConfig(configFile)
					if err != nil {
						log.Printf("[error] loading manifest source: %v", err.Error())
						return
					}
				}
			}
		} else {
			fmt.Printf("cli_config.yaml is using %v as source, ignoring CLI tag %v", manifest.SourceDirectory, config.CLIVersion)
		}

		pwd, err := os.Getwd()
		if err != nil {
			log.Printf("[error] %v", err.Error())
			return
		}
		if err := source.MoveToDirectory(filepath.Join(pwd, manifestsFilePath)); err != nil {
			log.Printf("[error] %v", err.Error())
			return
		}

		manifestsRepoPath, err := source.GetManifestPath()
		if err != nil {
			log.Printf("[error] %v", err.Error())
			return
		}

		if err := files.CreateIfNotExist(ParametersFilePath); err != nil {
			log.Println(err.Error())
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

		bld := manifest.CreateBuilder(loadedManifest)
		if err := bld.AddCommonComponents(); err != nil {
			log.Printf("[error] AddCommonComponents %v", err.Error())
			return
		}

		bld.AddOverlayContender(ArtifactRepositoryProvider)

		if err := addCloudProviderToManifestBuilder(Provider, bld); err != nil {
			log.Printf("[error] Adding Cloud Provider: %v", err.Error())
			return
		}

		if err := addDNSProviderToManifestBuilder(DNS, bld); err != nil {
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

			for i, d := range GPUDevicePlugins {
				if d == "nvidia" && Provider == "gke" {
					GPUDevicePlugins[i] = "gke"
				}
			}

			bld.AddOverlayContender(GPUDevicePlugins...)
		}

		bld.AddOverlayContender("cloud")

		if EnableHTTPS {
			bld.AddOverlayContender("https")
		}

		if Services != nil {
			if err := bld.AddComponent(Services...); err != nil {
				log.Printf("[error] Adding Components: %v", err.Error())
			}
		}

		if Provider == "eks" {
			overlay := strings.Join([]string{"cluster-autoscaler", "overlays", "eks"}, string(os.PathSeparator))
			if err := bld.AddOverlay(overlay); err != nil {
				log.Printf("[error] Adding overlay %v:\n", err.Error())
				return
			}
		}

		if !DisableServing {
			if err := bld.AddComponent("kfserving"); err != nil {
				log.Printf("[error] Adding component kfserving %v", err.Error())
				return
			}

			if !EnableCertManager {
				if err := bld.AddComponent("cert-manager"); err != nil {
					log.Printf(err.Error())
					return
				}
				bld.AddOverlayContender("self-signed")
			}
		}

		if ArtifactRepositoryProvider != artifactRepositoryProviderS3 {
			bld.AddOverlayContender("expose-minio")
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

		mergedParams.Merge(bld.GetYamls()...)

		mergedParams.Put("application.insecure", !EnableHTTPS)
		mergedParams.Put("application.provider", Provider)

		removeUneededArtifactRepositoryProviders(mergedParams)

		mergedParams.Sort()

		inputCommand := "# - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - -\n"
		inputCommand += "# Generated with Onepanel CLI \n"
		inputCommand += "# Command: opctl " + strings.Join(os.Args[1:], " ") + "\n"
		inputCommand += "# - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - -"
		if err := mergedParams.SetTopComment(inputCommand); err != nil {
			log.Printf("[error] setting comments: %v", err.Error())
			return
		}

		// Workflow Engine defaults to pns, but can be overwritten
		if err := mergedParams.Delete("workflowEngine"); err != nil {
			log.Printf("[error] %v", err.Error())
			return
		}

		// By default, random credentials for a Postgres database are generated
		if !Database {
			if err := mergedParams.Delete("database"); err != nil {
				log.Printf("[error] %v", err.Error())
				return
			}
		}

		paramsString, err := mergedParams.String()
		if err != nil {
			log.Printf("[error] unable to write params to a string")
			return
		}

		paramsFile, err := os.OpenFile(ParametersFilePath, os.O_RDWR|os.O_TRUNC, 0)
		if err != nil {
			log.Printf("Error opening parameters file: %v", err.Error())
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

	initCmd.Flags().StringVarP(&Provider, "provider", "p", "", "Cloud provider. Valid values: aks, gke, eks")
	initCmd.Flags().StringVarP(&DNS, "dns-provider", "d", "", "Provider for DNS. Valid values: azuredns, clouddns (google), cloudflare, route53")
	initCmd.Flags().StringVarP(&ArtifactRepositoryProvider, "artifact-repository-provider", "", "", "Object storage provider for storing artifacts. Valid value: s3, abs, gcs")
	initCmd.Flags().StringVarP(&ConfigurationFilePath, "config", "c", "config.yaml", "File path of the resulting config file")
	initCmd.Flags().StringVarP(&ParametersFilePath, "params", "e", "params.yaml", "File path of the resulting parameters file")
	initCmd.Flags().BoolVarP(&EnableEFKLogging, "enable-efk-logging", "", false, "Enable Elasticsearch, Fluentd and Kibana (EFK) logging")
	initCmd.Flags().BoolVarP(&EnableHTTPS, "enable-https", "", false, "Enable HTTPS scheme and redirect all requests to https://")
	initCmd.Flags().BoolVarP(&EnableCertManager, "enable-cert-manager", "", false, "Automatically create/renew TLS certs using Let's Encrypt")
	initCmd.Flags().BoolVarP(&EnableMetalLb, "enable-metallb", "", false, "Automatically create a LoadBalancer for non-cloud deployments.")
	initCmd.Flags().StringSliceVarP(&GPUDevicePlugins, "gpu-device-plugins", "", nil, "Install NVIDIA and/or AMD gpu device plugins. Valid values can be comma separated and are: amd, nvidia")
	initCmd.Flags().StringSliceVarP(&Services, "services", "", nil, "Install additional services. Valid values can be comma separated and are: modeldb")
	initCmd.Flags().BoolVarP(&Database, "database", "", false, "Use a pre-existing database, set up configuration in params.yaml")
	initCmd.Flags().BoolVarP(&DisableServing, "disable-serving", "", false, "Disable model serving")
}

func validateInput() error {
	if EnableCertManager && !EnableHTTPS {
		return fmt.Errorf("enable-https flag is required when enable-cert-manager is set")
	}

	if EnableCertManager && DNS == "" {
		return fmt.Errorf("dns-provider flag is required when enable-cert-manager is set")
	}

	if !EnableCertManager && DNS != "" {
		return fmt.Errorf("enable-cert-manager flag is required when dns-provider is set")
	}

	if err := validateProvider(Provider); err != nil {
		return err
	}

	if err := validateDNS(DNS); err != nil {
		return err
	}

	if err := validateGPUPlugins(GPUDevicePlugins); err != nil {
		return err
	}

	if err := validateArtifactRepositoryProvider(ArtifactRepositoryProvider); err != nil {
		return err
	}

	if err := validateServices(Services); err != nil {
		return err
	}

	for _, c := range Services {
		if c == "modeldb" {
			if ArtifactRepositoryProvider == artifactRepositoryProviderGcs {
				return fmt.Errorf("modeldb is currently not supported with GCS")
			}
		}
	}

	return nil
}

func validateProvider(prov string) error {
	if prov == "" {
		return errors.New("provider flag is required. Valid values: aks, gke, eks")
	}

	_, ok := providerProperties[prov]
	if !ok {
		return fmt.Errorf("'%v' is not a valid --provider value. Valid values: aks, gke, eks", prov)
	}

	return nil
}

func validateArtifactRepositoryProvider(arRepoProv string) error {
	if arRepoProv == "" {
		return errors.New("artifact-repository-provider flag is required. Valid value: s3, abs, gcs")
	}

	if arRepoProv == artifactRepositoryProviderS3 ||
		arRepoProv == artifactRepositoryProviderGcs ||
		arRepoProv == artifactRepositoryProviderAbs {
		return nil
	}

	return fmt.Errorf("'%v' is not a valid --artifact-repository-provider value. Valid value: s3, abs, gcs", arRepoProv)
}

func validateDNS(dns string) error {
	if dns != "route53" && dns != "" && dns != "clouddns" && dns != "azuredns" && dns != "cloudflare" {
		return fmt.Errorf("unsupported dns %v", dns)
	}

	return nil
}

func validateGPUPlugins(gpuPlugins []string) error {
	if gpuPlugins == nil {
		return nil
	}

	for _, p := range GPUDevicePlugins {
		if p != "amd" && p != "nvidia" {
			return fmt.Errorf("%v is not a valid --gpu-device-plugins value", p)
		}
	}

	return nil
}

func validateServices(services []string) error {
	if services == nil {
		return nil
	}

	for _, c := range services {
		if c != "modeldb" {
			return fmt.Errorf("%v is not a valid --component value", c)
		}
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

	if (provider == "minikube" || provider == "microk8s") && EnableMetalLb {
		if err := builder.AddComponent("metallb"); err != nil {
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

func removeUneededArtifactRepositoryProviders(mergedParams *util.DynamicYaml) {
	artifactRepoProviders := []string{artifactRepositoryProviderS3, artifactRepositoryProviderGcs, artifactRepositoryProviderAbs}
	var nodeKeyStr string
	for _, artRepoProv := range artifactRepoProviders {
		if ArtifactRepositoryProvider == artRepoProv {
			continue
		}
		nodeKeyStr = "artifactRepository." + artRepoProv
		nodeKey, _ := mergedParams.Get(nodeKeyStr)
		if nodeKey != nil {
			err := mergedParams.Delete(nodeKeyStr)
			if err != nil {
				log.Printf("error during init, artifact repository provider. %v", err.Error())
				return
			}
		}
	}

	parentValue := mergedParams.GetValue("artifactRepository")
	if parentValue != nil && len(parentValue.Content) == 0 {
		if err := mergedParams.Delete("artifactRepository"); err != nil {
			log.Printf("error during init, artifact repository provider. %v", err)
		}
	}
}
