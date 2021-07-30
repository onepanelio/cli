package cmd

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"github.com/onepanelio/cli/cloud/storage"
	"github.com/sethvargo/go-password/password"
	"golang.org/x/crypto/bcrypt"
	"io/ioutil"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/rand"
	"k8s.io/client-go/kubernetes"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	yaml2 "gopkg.in/yaml.v3"

	opConfig "github.com/onepanelio/cli/config"
	"github.com/onepanelio/cli/files"
	"github.com/onepanelio/cli/manifest"
	"github.com/onepanelio/cli/template"
	"github.com/onepanelio/cli/util"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v2"
	"sigs.k8s.io/kustomize/api/filesys"
	"sigs.k8s.io/kustomize/api/krusty"
	"sigs.k8s.io/kustomize/api/resmap"
)

// generateCmd represents the generate command
var generateCmd = &cobra.Command{
	Use:   "build",
	Short: "Builds application YAML for preview.",
	Run: func(cmd *cobra.Command, args []string) {
		configFilePath := "config.yaml"
		if len(args) > 1 {
			configFilePath = args[0]
		}

		k8sClient, err := util.NewKubernetesClient()
		if err != nil {
			fmt.Printf("Unable to get kubernetes client error: %v", err.Error())
			return
		}

		config, err := opConfig.FromFile(configFilePath)
		if err != nil {
			fmt.Printf("Unable to read configuration file: %v", err.Error())
			fmt.Println() // This gives us a newline as we get an extra "exiting" message
			return
		}

		kustomizeTemplate := TemplateFromSimpleOverlayedComponents(config.GetOverlayComponents(""))

		databaseConfig, err := GetDatabaseConfigurationFromCluster(k8sClient)
		if err != nil {
			fmt.Printf("[error] %v", err.Error())
			return
		}

		log.Printf("Building...")
		result, err := GenerateKustomizeResult(kustomizeTemplate, &GenerateKustomizeResultOptions{
			Config:   config,
			Database: databaseConfig,
		})
		if err != nil {
			fmt.Printf("%s\n", HumanizeKustomizeError(err))
			return
		}

		fmt.Printf("%v", result)
	},
}

func init() {
	rootCmd.AddCommand(generateCmd)
	generateCmd.Flags().BoolVarP(&Dev, "latest", "", false, "Sets conditions to allow development testing.")
}

// GenerateKustomizeResultOptions is configuration for the GenerateKustomizeResult function
type GenerateKustomizeResultOptions struct {
	Database *opConfig.Database
	Config   *opConfig.Config
}

// generateDatabaseConfiguration checks to see if database configuration is already present
// if not, it'll randomly generate some.
func generateDatabaseConfiguration(yaml *util.DynamicYaml, database *opConfig.Database) error {
	if yaml.HasKey("database") {
		return nil
	}

	if database == nil {
		dbPath := filepath.Join(".onepanel", "manifests", "cache", "common", "onepanel", "base", "vars.yaml")
		data, err := ioutil.ReadFile(dbPath)
		if err != nil {
			log.Fatal(err)
		}

		wrapper := &opConfig.DatabaseWrapper{}
		if err := yaml2.Unmarshal(data, wrapper); err != nil {
			log.Fatal(err)
		}

		database = wrapper.Database

		pass, err := password.Generate(16, 6, 0, false, false)
		if err != nil {
			return err
		}
		database.Password.Value = pass

		username, err := password.Generate(8, 6, 0, false, false)
		if err != nil {
			return err
		}
		database.Username.Value = "onepanel" + username
	}

	yaml.Put("database.host", database.Host.Value)
	yaml.Put("database.username", database.Username.Value)
	yaml.Put("database.password", database.Password.Value)
	yaml.Put("database.port", fmt.Sprintf(`"%s"`, database.Port.Value))
	yaml.Put("database.databaseName", database.DatabaseName.Value)
	yaml.Put("database.driverName", database.DriverName.Value)

	return nil
}

// GetDatabaseConfigurationFromCluster attempts to load the database configuration from a deployed cluster
// If there is no configuration (not found) no error is returned
func GetDatabaseConfigurationFromCluster(c *kubernetes.Clientset) (database *opConfig.Database, err error) {
	secret, err := c.CoreV1().Secrets("onepanel").Get(context.Background(), "onepanel", v1.GetOptions{})
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			return nil, nil
		}
		return
	}

	databaseUsername := string(secret.Data["databaseUsername"])
	databasePassword := string(secret.Data["databasePassword"])

	configMap, err := c.CoreV1().ConfigMaps("onepanel").Get(context.Background(), "onepanel", v1.GetOptions{})
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			return nil, nil
		}
		return
	}

	driverName := configMap.Data["databaseDriverName"]
	databaseHost := configMap.Data["databaseHost"]
	databaseName := configMap.Data["databaseName"]
	databasePort := configMap.Data["databasePort"]

	database = &opConfig.Database{
		Host:         opConfig.RequiredManifestVar(databaseHost),
		Username:     opConfig.RequiredManifestVar(databaseUsername),
		Password:     opConfig.RequiredManifestVar(databasePassword),
		Port:         opConfig.RequiredManifestVar(databasePort),
		DatabaseName: opConfig.RequiredManifestVar(databaseName),
		DriverName:   opConfig.RequiredManifestVar(driverName),
	}

	return
}

// GenerateKustomizeResult Given the path to the manifests, and a kustomize config, creates the final kustomization file.
// It does this by copying the manifests into a temporary directory, inserting the kustomize template
// and running the kustomize command
func GenerateKustomizeResult(kustomizeTemplate template.Kustomize, options *GenerateKustomizeResultOptions) (string, error) {
	config := *options.Config

	coreImageTag := opConfig.CoreImageTag
	coreImagePullPolicy := "IfNotPresent"
	coreUIImageTag := opConfig.CoreUIImageTag
	coreUIImagePullPolicy := "IfNotPresent"
	if Dev {
		coreImageTag = "latest"
		coreImagePullPolicy = "Always"
		coreUIImageTag = "latest"
		coreUIImagePullPolicy = "Always"
	} else if coreImageTag == "" {
		return "", fmt.Errorf("no version set. If you are running in dev mode, add the --latest flag")
	}

	yamlFile, err := util.LoadDynamicYamlFromFile(config.Spec.Params)
	if err != nil {
		return "", err
	}

	if err := manifest.Validate(yamlFile); err != nil {
		return "", err
	}

	manifestPath := config.Spec.ManifestsRepo
	localManifestsCopyPath := filepath.Join(".onepanel", "manifests", "cache")

	// Delete the local files if they exist
	if err := os.RemoveAll(localManifestsCopyPath); err != nil {
		return "", err
	}

	if err := files.CopyDir(manifestPath, localManifestsCopyPath); err != nil {
		return "", err
	}

	localKustomizePath := filepath.Join(localManifestsCopyPath, "kustomization.yaml")
	// Create will truncate the file if it exists
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

	domain := yamlFile.GetValue("application.domain").Value
	fqdn := yamlFile.GetValue("application.fqdn").Value
	cloudSettings, err := util.LoadDynamicYamlFromFile(filepath.Join(config.Spec.ManifestsRepo, "vars", "onepanel-config-map-hidden.env"))
	if err != nil {
		return "", err
	}

	defaultNamespace := yamlFile.GetValue("application.defaultNamespace").Value
	applicationAPIPath := cloudSettings.GetValue("applicationCloudApiPath").Value
	applicationAPIGRPCPort, _ := strconv.Atoi(cloudSettings.GetValue("applicationCloudApiGRPCPort").Value)
	applicationUIPath := cloudSettings.GetValue("applicationCloudUiPath").Value

	insecure, _ := strconv.ParseBool(yamlFile.GetValue("application.insecure").Value)
	httpScheme := "http://"
	wsScheme := "ws://"
	if !insecure {
		httpScheme = "https://"
		wsScheme = "wss://"
	}

	apiPath := httpScheme + fqdn + applicationAPIPath
	uiAPIPath := formatURLForUI(apiPath)
	uiAPIWsPath := formatURLForUI(wsScheme + fqdn + applicationAPIPath)

	yamlFile.PutWithSeparator("applicationApiUrl", uiAPIPath, ".")
	yamlFile.PutWithSeparator("applicationApiWsUrl", uiAPIWsPath, ".")
	yamlFile.PutWithSeparator("applicationApiPath", applicationAPIPath, ".")
	yamlFile.PutWithSeparator("applicationUiPath", applicationUIPath, ".")
	yamlFile.PutWithSeparator("applicationApiGrpcPort", applicationAPIGRPCPort, ".")
	yamlFile.PutWithSeparator("providerType", "cloud", ".")
	yamlFile.PutWithSeparator("onepanelApiUrl", apiPath, ".")

	yamlFile.PutWithSeparator("applicationCoreImageTag", coreImageTag, ".")
	yamlFile.PutWithSeparator("applicationCoreImagePullPolicy", coreImagePullPolicy, ".")

	yamlFile.PutWithSeparator("applicationCoreuiImageTag", coreUIImageTag, ".")
	yamlFile.PutWithSeparator("applicationCoreuiImagePullPolicy", coreUIImagePullPolicy, ".")

	applicationNodePoolOptionsConfigMapStr := generateApplicationNodePoolOptions(yamlFile.GetValue("application.nodePool"))
	yamlFile.PutWithSeparator("applicationNodePoolOptions", applicationNodePoolOptionsConfigMapStr, ".")

	provider := yamlFile.GetValue("application.provider").Value
	if provider == "minikube" || provider == "microk8s" {
		metalLbAddressesConfigMapStr := generateMetalLbAddresses(yamlFile.GetValue("metalLb.addresses").Content)
		yamlFile.PutWithSeparator("metalLbAddresses", metalLbAddressesConfigMapStr, ".")

		metalLbSecretKey, err := bcrypt.GenerateFromPassword([]byte(rand.String(128)), bcrypt.DefaultCost)
		if err != nil {
			return "", err
		}
		yamlFile.PutWithSeparator("metalLbSecretKey", base64.StdEncoding.EncodeToString(metalLbSecretKey), ".")
	}

	_, artifactRepositoryNode := yamlFile.Get("artifactRepository")
	artifactRepositoryConfig := storage.ArtifactRepositoryProvider{}
	err = artifactRepositoryNode.Decode(&artifactRepositoryConfig)
	if err != nil {
		return "", err
	}
	if artifactRepositoryConfig.S3 != nil {
		artifactRepositoryConfig.S3.AccessKeySecret.Key = "artifactRepositoryS3AccessKey"
		artifactRepositoryConfig.S3.AccessKeySecret.Name = "$(artifactRepositoryS3AccessKeySecretName)"
		artifactRepositoryConfig.S3.SecretKeySecret.Key = "artifactRepositoryS3SecretKey"
		artifactRepositoryConfig.S3.SecretKeySecret.Name = "$(artifactRepositoryS3SecretKeySecretName)"
		artifactRepositoryConfig.S3.PublicEndpoint = artifactRepositoryConfig.S3.Endpoint
		artifactRepositoryConfig.S3.PublicInsecure = artifactRepositoryConfig.S3.Insecure

		yamlStr, err := artifactRepositoryConfig.S3.MarshalToYaml()
		if err != nil {
			return "", err
		}
		yamlFile.Put("artifactRepositoryProvider", yamlStr)
		yamlFile.Put("artifactRepository.s3.region", artifactRepositoryConfig.S3.Region)
	} else if artifactRepositoryConfig.GCS != nil {
		accessKey := artifactRepositoryConfig.GCS.Bucket
		randomSecret, err := util.RandASCIIString(16)
		if err != nil {
			return "", err
		}

		artifactRepositoryConfig.S3 = &storage.ArtifactRepositoryS3Provider{
			KeyFormat:      artifactRepositoryConfig.GCS.KeyFormat,
			Bucket:         artifactRepositoryConfig.GCS.Bucket,
			Endpoint:       fmt.Sprintf("minio-gateway.%v.svc.cluster.local:9000", defaultNamespace),
			PublicEndpoint: fmt.Sprintf("sys-storage-%v.%v", defaultNamespace, domain),
			Insecure:       true,
			PublicInsecure: insecure,
			AccessKeySecret: storage.ArtifactRepositorySecret{
				Key:  "artifactRepositoryS3AccessKey",
				Name: accessKey,
			},
			SecretKeySecret: storage.ArtifactRepositorySecret{
				Key:  "artifactRepositoryS3SecretKey",
				Name: randomSecret,
			},
		}
		yamlStr, err := artifactRepositoryConfig.S3.MarshalToYaml()
		if err != nil {
			return "", err
		}

		yamlFile.Put("artifactRepositoryProvider", yamlStr)
		yamlFile.Put("artifactRepository.s3.accessKey", accessKey)
		yamlFile.Put("artifactRepository.s3.region", "us-west-2")
		yamlFile.Put("artifactRepository.s3.secretKey", randomSecret)
		yamlFile.Put("artifactRepository.s3.bucket", artifactRepositoryConfig.GCS.Bucket)
		yamlFile.Put("artifactRepository.s3.endpoint", artifactRepositoryConfig.S3.Endpoint)
		yamlFile.Put("artifactRepository.s3.publicEndpoint", artifactRepositoryConfig.S3.PublicEndpoint)
		yamlFile.Put("artifactRepository.s3.insecure", "true")
		yamlFile.Put("artifactRepositoryServiceAccountKey", base64.StdEncoding.EncodeToString([]byte(artifactRepositoryConfig.GCS.ServiceAccountKey)))
	} else if artifactRepositoryConfig.ABS != nil {
		artifactRepositoryConfig.S3 = &storage.ArtifactRepositoryS3Provider{
			KeyFormat:      artifactRepositoryConfig.ABS.KeyFormat,
			Bucket:         artifactRepositoryConfig.ABS.Container,
			Endpoint:       fmt.Sprintf("minio-gateway.%v.svc.cluster.local:9000", defaultNamespace),
			PublicEndpoint: fmt.Sprintf("sys-storage-%v.%v", defaultNamespace, domain),
			Insecure:       true,
			PublicInsecure: insecure,
			AccessKeySecret: storage.ArtifactRepositorySecret{
				Key:  "artifactRepositoryS3AccessKey",
				Name: "$(artifactRepositoryS3AccessKey)",
			},
			SecretKeySecret: storage.ArtifactRepositorySecret{
				Key:  "artifactRepositoryS3SecretKey",
				Name: "$(artifactRepositoryS3SecretKeySecretName)",
			},
		}
		yamlStr, err := artifactRepositoryConfig.S3.MarshalToYaml()
		if err != nil {
			return "", err
		}

		yamlFile.Put("artifactRepositoryProvider", yamlStr)
		yamlFile.Put("artifactRepository.s3.accessKey", "placeholder")
		yamlFile.Put("artifactRepository.s3.secretKey", "placeholder")
		yamlFile.Put("artifactRepository.s3.bucket", "bucket-name")
		yamlFile.Put("artifactRepository.s3.region", "us-west-2")
		yamlFile.Put("artifactRepository.s3.endpoint", artifactRepositoryConfig.S3.Endpoint)
		yamlFile.Put("artifactRepository.s3.publicEndpoint", artifactRepositoryConfig.S3.PublicEndpoint)
		yamlFile.Put("artifactRepository.s3.insecure", "true")
	} else {
		return "", errors.New("unsupported artifactRepository configuration")
	}

	// Check if workflowEngineContainerRuntimeExecutor is in the vars.
	// If it is, leave it. If it is not, load it from the manifests and use the default
	if !yamlFile.HasKey("workflowEngine.containerRuntimeExecutor") {
		argoVarsYaml, err := util.LoadDynamicYamlFromFile(filepath.Join(".onepanel", "manifests", "cache", "common", "argo", "base", "vars.yaml"))
		if err != nil {
			return "", err
		}

		_, valueNode := argoVarsYaml.Get("workflowEngine.containerRuntimeExecutor.default")
		if valueNode == nil {
			return "", fmt.Errorf("workflowEngine.containerRuntimeExecutor.default does not exist in manifests")
		}

		yamlFile.Put("workflowEngineContainerRuntimeExecutor", valueNode.Value)
	}

	if err := generateDatabaseConfiguration(yamlFile, options.Database); err != nil {
		return "", err
	}

	flatMap := yamlFile.FlattenToKeyValue(util.LowerCamelCaseFlatMapKeyFormatter)
	if err := mapLinkedVars(flatMap, localManifestsCopyPath, &config, true); err != nil {
		return "", err
	}

	//Read workflow-config-map-hidden for the rest of the values
	workflowEnvHiddenPath := filepath.Join(localManifestsCopyPath, "vars", "workflow-config-map-hidden.env")
	workflowEnvCont, workflowEnvFileErr := ioutil.ReadFile(workflowEnvHiddenPath)
	if workflowEnvFileErr != nil {
		return "", workflowEnvFileErr
	}
	workflowEnvContStr := string(workflowEnvCont)
	//Add these keys and values
	for _, line := range strings.Split(workflowEnvContStr, "\n") {
		line = strings.ReplaceAll(line, "\r", "")
		keyValArr := strings.Split(line, "=")
		if len(keyValArr) != 2 {
			continue
		}
		k := keyValArr[0]

		// Do not include the extra S3 parameters if they are not set in the params.yaml
		if artifactRepositoryConfig.S3 == nil {
			if strings.Contains(k, "S3") {
				continue
			}
		}
		v := keyValArr[1]
		flatMap[k] = v
	}

	//Replace artifactRepository placeholders for S3
	if artifactRepositoryConfig.S3 != nil {
		artifactRepositoryS3AccessKeySecretName, ok := flatMap["artifactRepositoryS3AccessKeySecretName"].(string)
		if !ok {
			return "", fmt.Errorf("missing 'artifactRepositoryS3AccessKeySecretName'")
		}
		artifactRepositoryS3SecretKeySecretName, ok := flatMap["artifactRepositoryS3SecretKeySecretName"].(string)
		if !ok {
			return "", fmt.Errorf("missing 'artifactRepositoryS3SecretKeySecretName'")
		}
		artifactRepositoryConfig.S3.AccessKeySecret.Name = artifactRepositoryS3AccessKeySecretName
		artifactRepositoryConfig.S3.SecretKeySecret.Name = artifactRepositoryS3SecretKeySecretName
		yamlStr, err := artifactRepositoryConfig.S3.MarshalToYaml()
		if err != nil {
			return "", err
		}
		flatMap["artifactRepositoryProvider"] = yamlStr
	}

	//Write to env files
	//workflow-config-map.env
	//Set extra values for S3 specific configuration.
	if artifactRepositoryConfig.ABS != nil {
		missingKeys := yamlFile.FindMissingKeys("artifactRepository.s3.bucket", "artifactRepository.s3.endpoint", "artifactRepository.s3.insecure")
		if len(missingKeys) == 0 {
			paramsPath := filepath.Join(localManifestsCopyPath, "vars", "workflow-config-map.env")
			//Clear previous env file - create truncates if it exists
			paramsFile, err := os.Create(paramsPath)
			if err != nil {
				return "", err
			}
			var stringToWrite = fmt.Sprintf("%v=%v\n%v=%v\n%v=%v\n%v=%v\n",
				"artifactRepositoryBucket", flatMap["artifactRepositoryS3Bucket"],
				"artifactRepositoryEndpoint", flatMap["artifactRepositoryS3Endpoint"],
				"artifactRepositoryInsecure", flatMap["artifactRepositoryS3Insecure"],
			)
			_, err = paramsFile.WriteString(stringToWrite)
			if err != nil {
				return "", err
			}
		} else {
			missingKeysMessage := strings.Join(missingKeys, ", ")
			log.Fatalf("missing required values in params.yaml: %v", missingKeysMessage)
		}
	} else if artifactRepositoryConfig.S3 != nil && artifactRepositoryConfig.GCS == nil {
		missingKeys := yamlFile.FindMissingKeys("artifactRepository.s3.bucket", "artifactRepository.s3.endpoint", "artifactRepository.s3.insecure", "artifactRepository.s3.region")
		if len(missingKeys) == 0 {
			paramsPath := filepath.Join(localManifestsCopyPath, "vars", "workflow-config-map.env")
			//Clear previous env file - create truncates if it exists
			paramsFile, err := os.Create(paramsPath)
			if err != nil {
				return "", err
			}
			var stringToWrite = fmt.Sprintf("%v=%v\n%v=%v\n%v=%v\n%v=%v\n",
				"artifactRepositoryBucket", flatMap["artifactRepositoryS3Bucket"],
				"artifactRepositoryEndpoint", flatMap["artifactRepositoryS3Endpoint"],
				"artifactRepositoryInsecure", flatMap["artifactRepositoryS3Insecure"],
				"artifactRepositoryRegion", flatMap["artifactRepositoryS3Region"],
			)
			_, err = paramsFile.WriteString(stringToWrite)
			if err != nil {
				return "", err
			}
		} else {
			missingKeysMessage := strings.Join(missingKeys, ", ")
			log.Fatalf("missing required values in params.yaml: %v", missingKeysMessage)
		}
	}
	//logging-config-map.env, optional component
	if yamlFile.HasKeys("logging.image", "logging.volumeStorage") {
		paramsPath := filepath.Join(localManifestsCopyPath, "vars", "logging-config-map.env")
		//Clear previous env file
		paramsFile, err := os.Create(paramsPath)
		if err != nil {
			return "", err
		}
		var stringToWrite = fmt.Sprintf("%v=%v\n%v=%v\n",
			"loggingImage", flatMap["loggingImage"],
			"loggingVolumeStorage", flatMap["loggingVolumeStorage"],
		)
		_, err = paramsFile.WriteString(stringToWrite)
		if err != nil {
			return "", err
		}
	}
	//onepanel-config-map.env
	//Clear previous env file
	paramsPath := filepath.Join(localManifestsCopyPath, "vars", "onepanel-config-map.env")
	paramsFile, err := os.Create(paramsPath)
	if err != nil {
		return "", err
	}
	var stringToWrite = fmt.Sprintf("%v=%v\n",
		"applicationDefaultNamespace", flatMap["applicationDefaultNamespace"],
	)
	if _, err := paramsFile.WriteString(stringToWrite); err != nil {
		return "", err
	}

	//Write to secret files
	var secretKeysValues []string
	artifactRepoSecretPlaceholder := "$(artifactRepositoryProviderSecret)"
	if yamlFile.HasKey("artifactRepository.s3") {
		missingKeys := yamlFile.FindMissingKeys("artifactRepository.s3.accessKey", "artifactRepository.s3.secretKey")
		if len(missingKeys) == 0 {
			secretKeysValues = append(secretKeysValues, "artifactRepositoryS3AccessKey", "artifactRepositoryS3SecretKey")

			artifactRepoS3Secret := fmt.Sprintf(
				"artifactRepositoryS3AccessKey: %v"+
					"\n  artifactRepositoryS3SecretKey: %v",
				flatMap["artifactRepositoryS3AccessKey"], flatMap["artifactRepositoryS3SecretKey"])

			err = replacePlaceholderForSecretManiFile(localManifestsCopyPath, artifactRepoSecretPlaceholder, artifactRepoS3Secret)
			if err != nil {
				return "", err
			}
		} else {
			missingKeysMessage := strings.Join(missingKeys, ", ")
			log.Fatalf("Missing required values in params.yaml: %v", missingKeysMessage)
		}
	}

	//To properly replace $(applicationDefaultNamespace), we need to update it in quite a few files.
	//Find those files
	listOfFiles, errorWalking := FilePathWalkDir(localManifestsCopyPath)
	if errorWalking != nil {
		return "", err
	}

	if err := replaceVariables(flatMap, listOfFiles); err != nil {
		return "", err
	}

	//Update the values in those files
	rm, err := runKustomizeBuild(localManifestsCopyPath)
	if err != nil {
		return "", err
	}
	kustYaml, err := rm.AsYaml()

	return string(kustYaml), nil
}

func replacePlaceholderForSecretManiFile(localManifestsCopyPath string, artifactRepoSecretPlaceholder string, artifactRepoSecretVal string) error {
	//Path to secrets file
	secretsPath := filepath.Join(localManifestsCopyPath, "common", "onepanel", "base", "secret-onepanel-defaultnamespace.yaml")
	//Read the file, replace the specific value, write the file back
	secretFileContent, secretFileOpenErr := ioutil.ReadFile(secretsPath)
	if secretFileOpenErr != nil {
		return secretFileOpenErr
	}
	secretFileContentStr := string(secretFileContent)
	if strings.Contains(secretFileContentStr, artifactRepoSecretPlaceholder) {
		secretFileContentStr = strings.Replace(secretFileContentStr, artifactRepoSecretPlaceholder, artifactRepoSecretVal, 1)
		writeFileErr := ioutil.WriteFile(secretsPath, []byte(secretFileContentStr), 0644)
		if writeFileErr != nil {
			return writeFileErr
		}
	} else {
		fmt.Printf("Key: %v not present in %v, not used.\n", artifactRepoSecretPlaceholder, secretsPath)
	}
	return nil
}

func TemplateFromSimpleOverlayedComponents(comps []*opConfig.SimpleOverlayedComponent) template.Kustomize {
	k := template.Kustomize{
		ApiVersion:     "kustomize.config.k8s.io/v1beta1",
		Kind:           "Kustomization",
		Resources:      make([]string, 0),
		Configurations: []string{filepath.Join("configs/varreference.yaml")},
	}

	for _, overlayComponent := range comps {
		for _, item := range overlayComponent.PartsSkipFirst() {
			k.Resources = append(k.Resources, *item)
		}
	}

	return k
}

// FilePathWalkDir goes through a directory and finds all of the files under it, including subdirectories
func FilePathWalkDir(root string) ([]string, error) {
	var filesFound []string
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if !info.IsDir() {
			if !strings.Contains(path, ".git") {
				filesFound = append(filesFound, path)
			}
		}
		return nil
	})
	return filesFound, err
}

func formatURLForUI(url string) string {
	result := strings.Replace(url, "/", `\/`, -1)
	result = strings.Replace(result, ".", `\.`, -1)
	result = strings.Replace(result, ":", `\:`, -1)

	return result
}

func runKustomizeBuild(path string) (rm resmap.ResMap, err error) {
	fSys := filesys.MakeFsOnDisk()
	opts := krusty.MakeDefaultOptions()

	k := krusty.MakeKustomizer(opts)

	rm, err = k.Run(fSys, path)
	if err != nil {
		return nil, fmt.Errorf("Kustomizer Run for path '%s' failed: %s", path, err)
	}

	return rm, nil
}

func generateApplicationNodePoolOptions(nodePoolData *yaml2.Node) string {
	nodePool := struct {
		Options []map[string]interface{}
	}{}
	if err := nodePoolData.Decode(&nodePool); err != nil {
		log.Println(err)
		return ""
	}

	nodePoolOptions, err := yaml2.Marshal(nodePool.Options)
	if err != nil {
		log.Println(err)
		return ""
	}

	nodePoolOptionsStr := "|\n"
	for _, line := range strings.Split(string(nodePoolOptions), "\n") {
		nodePoolOptionsStr += fmt.Sprintf("    %v\n", line)
	}

	return nodePoolOptionsStr
}

func generateMetalLbAddresses(nodePoolData []*yaml2.Node) string {
	applicationNodePoolOptions := []string{""}
	var appendStr string
	for idx, poolNode := range nodePoolData {
		if poolNode.Tag == "!!str" {
			if idx > 0 {
				//yaml spacing
				appendStr = "      "
			}
			appendStr += "- " + poolNode.Value + "\n"
			applicationNodePoolOptions = append(applicationNodePoolOptions, appendStr)
			appendStr = ""
		}
	}
	return strings.Join(applicationNodePoolOptions, "")
}

// mapLinkedVars goes through the `default-vars.yaml` files which map variables from already existing variables
// and set those variable values. If the value is already in the mapping, it is not mapped to the default.
func mapLinkedVars(mapping map[string]interface{}, manifestPath string, config *opConfig.Config, replace bool) error {
	paths := make([]string, 0)
	pathsAdded := make(map[string]bool)

	for _, component := range config.Spec.Components {
		if !pathsAdded["modeldb"] && strings.Contains(component, "modeldb") {
			paths = append(paths, filepath.Join(manifestPath, "modeldb", "base", "default-vars.yaml"))
			pathsAdded["modeldb"] = true
		}
	}

	for _, component := range config.Spec.Overlays {
		if !pathsAdded["artifact-repository"] && strings.Contains(component, filepath.Join("artifact-repository", "overlays", "abs")) {
			paths = append(paths, filepath.Join(manifestPath, "common", "artifact-repository", "overlays", "abs", "default-vars.yaml"))
			pathsAdded["artifact-repository"] = true
		}
	}

	for _, path := range paths {
		loadedMapping, err := util.LoadDynamicYamlFromFile(path)
		if err != nil {
			return err
		}

		flatMappedVars := loadedMapping.Flatten(util.LowerCamelCaseFlatMapKeyFormatter)
		for key, valueNode := range flatMappedVars {
			// Skip if key already exists
			if !replace {
				if _, ok := mapping[key]; ok {
					continue
				}
			}

			valueKey := util.LowerCamelCaseStringFormat(valueNode.Value.Value, ".")
			value, ok := mapping[valueKey]
			if !ok {
				return fmt.Errorf("unknown key %v", valueKey)
			}

			mapping[key] = value
		}
	}

	return nil
}

// HumanizeKustomizeError takes errors returned from GenerateKustomizeResult and returns them in a human friendly string
func HumanizeKustomizeError(err error) string {
	if paramsError, ok := err.(*manifest.ParamsError); ok {
		switch paramsError.ErrorType {
		case "missing":
			return fmt.Sprintf("%s is missing in your params.yaml", paramsError.Key)
		case "parameter":
			return fmt.Sprintf("%s can not be '%s', please enter a different value for %v. %v", paramsError.Key, *paramsError.Value, paramsError.ShortKey, paramsError.ValidationMessage)
		case "blank":
			return fmt.Sprintf("%s can not be blank, please use a different %v in your params.yaml", paramsError.Key, paramsError.ShortKey)
		case "reserved":
			return fmt.Sprintf("%s can not be '%v' please use a different %v in your params.yaml", paramsError.Key, *paramsError.Value, paramsError.ShortKey)
		}
	}

	return fmt.Sprintf("Error generating result: %v", err.Error())
}

// replaceVariable will go through the variables in flatMap and replace any instances of it in fileContent
// the resulting modified content is returned
func replaceVariable(flatMap map[string]interface{}, fileContent []byte) []byte {
	manifestFileContentStr := string(fileContent)
	useStr := ""
	rawStr := ""

	for key := range flatMap {
		valueBool, okBool := flatMap[key].(bool)
		if okBool {
			useStr = strconv.FormatBool(valueBool)
			rawStr = strconv.FormatBool(valueBool)
		} else {
			valueInt, okInt := flatMap[key].(int)
			if okInt {
				useStr = "\"" + strconv.FormatInt(int64(valueInt), 10) + "\""
				rawStr = strconv.FormatInt(int64(valueInt), 10)
			} else {
				valueStr, ok := flatMap[key].(string)
				if !ok {
					log.Fatal("Unrecognized value in flatmap. Check type assertions.")
				}
				useStr = valueStr
				rawStr = valueStr
			}
		}
		oldString := "$(" + key + ")"
		if strings.Contains(manifestFileContentStr, key) {
			manifestFileContentStr = strings.Replace(manifestFileContentStr, oldString, useStr, -1)
		}
		oldRawString := "$raw(" + key + ")"
		if strings.Contains(manifestFileContentStr, key) {
			manifestFileContentStr = strings.Replace(manifestFileContentStr, oldRawString, rawStr, -1)
		}

		oldBase64String := "$base64(" + key + ")"
		if strings.Contains(manifestFileContentStr, key) {
			base64Value := base64.StdEncoding.EncodeToString([]byte(rawStr))
			manifestFileContentStr = strings.Replace(manifestFileContentStr, oldBase64String, base64Value, -1)
		}
	}

	return []byte(manifestFileContentStr)
}

// replaceVariables will go through the variables in flatMap and replace any instances of it in any of the files in filePaths
// the file content is replaced, no backups are made.
func replaceVariables(flatMap map[string]interface{}, filePaths []string) error {
	for _, filePath := range filePaths {
		manifestFileContent, manifestFileOpenErr := ioutil.ReadFile(filePath)
		if manifestFileOpenErr != nil {
			return manifestFileOpenErr
		}

		manifestFileContentStr := replaceVariable(flatMap, manifestFileContent)
		if err := ioutil.WriteFile(filePath, manifestFileContentStr, 0644); err != nil {
			return err
		}
	}

	return nil
}
