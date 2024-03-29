package cmd

import (
	"fmt"
	"github.com/onepanelio/cli/cloud/storage"
	opConfig "github.com/onepanelio/cli/config"
	"github.com/onepanelio/cli/util"
	"github.com/spf13/cobra"
	"log"
	"strings"
)

var appCmd = &cobra.Command{
	Use:     "app",
	Short:   "Various app functions.",
	Long:    "Inspect and execute various app functions..",
	Example: "app",
	Run:     func(cmd *cobra.Command, args []string) {},
}

var statusCmd = &cobra.Command{
	Use:     "status",
	Short:   "Check deployment status.",
	Long:    "Check deployment status by checking pods statuses.",
	Example: "status",
	Run: func(cmd *cobra.Command, args []string) {
		k8sClient, err := util.NewKubernetesClient()
		if err != nil {
			fmt.Printf("Unable to create kubernetes client: error %v", err.Error())
			return
		}

		configFilePath := "config.yaml"
		config, err := opConfig.FromFile(configFilePath)
		if err != nil {
			fmt.Printf("Unable to read configuration file: %v", err.Error())
			return
		}
		yamlFile, err := util.LoadDynamicYamlFromFile(config.Spec.Params)
		if err != nil {
			fmt.Println("Error parsing configuration file.")
			return
		}

		ready, err := util.NamespacesExist(k8sClient, util.NamespacesToCheck(yamlFile)...)
		if err != nil {
			flatMap := yamlFile.FlattenToKeyValue(util.AppendDotFlatMapKeyFormatter)
			provider, providerErr := util.GetYamlStringValue(flatMap, "application.provider")
			if providerErr != nil {
				fmt.Printf("Unable to read application.provider from params.yaml %v", providerErr.Error())
				return
			}
			if provider == nil {
				fmt.Printf("application.provider is not set in params.yaml")
				return
			}

			if *provider == "microk8s" {
				fmt.Printf("Unable to connect to cluster. Make sure you are running with \nKUBECONFIG=./kubeconfig opctl app status\nError: %v", err.Error())
				return
			}

			fmt.Println(err.Error())
			return
		}

		if ready {
			fmt.Println("Your deployment is ready.")
		} else {
			fmt.Println("Your deployment is NOT ready; not all Pods are running. To view all Pods:")
			fmt.Println("$ kubectl get pods -A")
		}

		// Get cluster deployment URL
		url, err := util.GetDeployedWebURL(yamlFile)
		if err != nil {
			fmt.Printf("[error] Unable to get deployed url from configuration: %v", err.Error())
			return
		}

		util.PrintClusterNetworkInformation(k8sClient, url)

		_, artifactRepositoryNode := yamlFile.Get("artifactRepository")
		artifactRepositoryConfig := storage.ArtifactRepositoryProvider{}
		if err := artifactRepositoryNode.Decode(&artifactRepositoryConfig); err != nil {
			fmt.Printf("Unable to check artifactRepository configuration. Original error: %v", err.Error())
			return
		}

		defaultNamespace := yamlFile.GetValue("application.defaultNamespace").Value
		domain := yamlFile.GetValue("application.domain").Value
		isHTTPS := strings.ToLower(yamlFile.GetValue("application.insecure").Value) == "false"

		if err := artifactRepositoryConfig.Load(k8sClient, defaultNamespace); err != nil {
			fmt.Println(err)
			return
		}

		minioClient, err := artifactRepositoryConfig.MinioClient(defaultNamespace, domain, isHTTPS)
		if err != nil {
			log.Printf("Unable to run tests on storage. Original error %v", err.Error())
			return
		}

		bucket, err := artifactRepositoryConfig.Bucket()
		if err != nil {
			fmt.Printf(err.Error())
			return
		}

		if err := storage.TestMinioStorageConnection(minioClient, bucket); err != nil {
			fmt.Printf("ArtifactRepository tests: Failed\n")
			fmt.Printf("  " + err.Error())
		} else {
			fmt.Printf("ArtifactRepository tests: Successful\n")
		}
	},
}

func init() {
	rootCmd.AddCommand(appCmd)
	appCmd.AddCommand(statusCmd)
}
