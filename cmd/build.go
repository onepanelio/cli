package cmd

import (
	"encoding/base64"
	"errors"
	"fmt"
	v1 "github.com/onepanelio/core/pkg"
	"golang.org/x/crypto/bcrypt"
	"io/ioutil"
	"k8s.io/apimachinery/pkg/util/rand"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	yaml2 "gopkg.in/yaml.v3"

	"sigs.k8s.io/kustomize/api/filesys"
	"sigs.k8s.io/kustomize/api/krusty"
	"sigs.k8s.io/kustomize/api/resmap"
	"sigs.k8s.io/kustomize/api/types"

	opConfig "github.com/onepanelio/cli/config"
	"github.com/onepanelio/cli/files"
	"github.com/onepanelio/cli/manifest"
	"github.com/onepanelio/cli/template"
	"github.com/onepanelio/cli/util"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v2"
)

// generateCmd represents the generate command
var generateCmd = &cobra.Command{
	Use:   "build",
	Short: "Builds application YAML for preview.",
	Run: func(cmd *cobra.Command, args []string) {
		configFilePath := "config.yaml"

		if len(args) > 1 {
			configFilePath = args[0]
			return
		}

		config, err := opConfig.FromFile(configFilePath)
		if err != nil {
			fmt.Printf("Unable to read configuration file: %v", err.Error())
			fmt.Println() // This gives us a newline as we get an extra "exiting" message
			return
		}

		kustomizeTemplate := TemplateFromSimpleOverlayedComponents(config.GetOverlayComponents(""))

		log.Printf("Building...")
		result, err := GenerateKustomizeResult(*config, kustomizeTemplate)
		if err != nil {
			fmt.Printf("%s\n", HumanizeKustomizeError(err))
			return
		}

		fmt.Printf("%v", result)
	},
}

func init() {
	rootCmd.AddCommand(generateCmd)
	generateCmd.Flags().BoolVarP(&Dev, "dev", "", false, "Sets conditions to allow development testing.")
}

// Given the path to the manifests, and a kustomize config, creates the final kustomization file.
// It does this by copying the manifests into a temporary directory, inserting the kustomize template
// and running the kustomize command
func GenerateKustomizeResult(config opConfig.Config, kustomizeTemplate template.Kustomize) (string, error) {
	yamlFile, err := util.LoadDynamicYamlFromFile(config.Spec.Params)
	if err != nil {
		return "", err
	}

	if err := manifest.Validate(yamlFile); err != nil {
		return "", err
	}

	manifestPath := config.Spec.ManifestsRepo
	localManifestsCopyPath := filepath.Join(".onepanel", "manifests", "cache")

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

	fqdn := yamlFile.GetValue("application.fqdn").Value
	cloudSettings, err := util.LoadDynamicYamlFromFile(filepath.Join(config.Spec.ManifestsRepo, "vars", "onepanel-config-map-hidden.env"))
	if err != nil {
		return "", err
	}

	applicationApiPath := cloudSettings.GetValue("applicationCloudApiPath").Value
	applicationApiGrpcPort, _ := strconv.Atoi(cloudSettings.GetValue("applicationCloudApiGRPCPort").Value)
	applicationUiPath := cloudSettings.GetValue("applicationCloudUiPath").Value

	insecure, _ := strconv.ParseBool(yamlFile.GetValue("application.insecure").Value)
	httpScheme := "http://"
	wsScheme := "ws://"
	if !insecure {
		httpScheme = "https://"
		wsScheme = "wss://"
	}

	apiPath := httpScheme + fqdn + applicationApiPath
	uiApiPath := formatUrlForUi(apiPath)
	uiApiWsPath := formatUrlForUi(wsScheme + fqdn + applicationApiPath)

	yamlFile.PutWithSeparator("applicationApiUrl", uiApiPath, ".")
	yamlFile.PutWithSeparator("applicationApiWsUrl", uiApiWsPath, ".")
	yamlFile.PutWithSeparator("applicationApiPath", applicationApiPath, ".")
	yamlFile.PutWithSeparator("applicationUiPath", applicationUiPath, ".")
	yamlFile.PutWithSeparator("applicationApiGrpcPort", applicationApiGrpcPort, ".")
	yamlFile.PutWithSeparator("providerType", "cloud", ".")
	yamlFile.PutWithSeparator("onepanelApiUrl", apiPath, ".")

	coreImageTag := opConfig.CoreImageTag
	coreImagePullPolicy := "IfNotPresent"
	coreUiImageTag := opConfig.CoreUIImageTag
	coreUiImagePullPolicy := "IfNotPresent"
	if Dev {
		coreImageTag = "dev"
		coreImagePullPolicy = "Always"
		coreUiImageTag = "dev"
		coreUiImagePullPolicy = "Always"
	}
	yamlFile.PutWithSeparator("applicationCoreImageTag", coreImageTag, ".")
	yamlFile.PutWithSeparator("applicationCoreImagePullPolicy", coreImagePullPolicy, ".")

	yamlFile.PutWithSeparator("applicationCoreuiImageTag", coreUiImageTag, ".")
	yamlFile.PutWithSeparator("applicationCoreuiImagePullPolicy", coreUiImagePullPolicy, ".")

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
	artifactRepositoryConfig := v1.ArtifactRepositoryProvider{}
	err = artifactRepositoryNode.Decode(&artifactRepositoryConfig)
	if err != nil {
		return "", err
	}
	if artifactRepositoryConfig.S3 != nil {
		artifactRepositoryConfig.S3.AccessKeySecret.Key = "artifactRepositoryS3AccessKey"
		artifactRepositoryConfig.S3.AccessKeySecret.Name = "$(artifactRepositoryS3AccessKeySecretName)"
		artifactRepositoryConfig.S3.SecretKeySecret.Key = "artifactRepositoryS3SecretKey"
		artifactRepositoryConfig.S3.SecretKeySecret.Name = "$(artifactRepositoryS3SecretKeySecretName)"
		yamlStr, err := artifactRepositoryConfig.S3.MarshalToYaml()
		if err != nil {
			return "", err
		}
		yamlFile.Put("artifactRepositoryProvider", yamlStr)
	} else if artifactRepositoryConfig.GCS != nil {
		yamlConfigMap, err := artifactRepositoryConfig.GCS.MarshalToYaml()
		if err != nil {
			return "", err
		}

		yamlFile.Put("artifactRepositoryProvider", yamlConfigMap)
	} else {
		return "", errors.New("unsupported artifactRepository configuration")
	}
	flatMap := yamlFile.FlattenToKeyValue(util.LowerCamelCaseFlatMapKeyFormatter)
	if err := mapLinkedVars(flatMap, localManifestsCopyPath, &config); err != nil {
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
		keyValArr := strings.Split(line, "=")
		if len(keyValArr) != 2 {
			continue
		}
		k := keyValArr[0]
		/**
		Do not include the extra S3 parameters if they are not set in the params.yaml
		*/
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
			if err != nil {
				return "", err
			}
		}
		artifactRepositoryS3SecretKeySecretName, ok := flatMap["artifactRepositoryS3SecretKeySecretName"].(string)
		if !ok {
			if err != nil {
				return "", err
			}
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
	if artifactRepositoryConfig.S3 != nil && artifactRepositoryConfig.GCS == nil {
		if yamlFile.HasKeys("artifactRepository.s3.bucket", "artifactRepository.s3.endpoint", "artifactRepository.s3.insecure", "artifactRepository.s3.region") {
			//Clear previous env file
			paramsPath := filepath.Join(localManifestsCopyPath, "vars", "workflow-config-map.env")
			if _, err := files.DeleteIfExists(paramsPath); err != nil {
				return "", err
			}
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
			log.Fatal("Missing required values in params.yaml, artifactRepository. Check bucket, endpoint, or insecure.")
		}
	}
	//logging-config-map.env, optional component
	if yamlFile.HasKeys("logging.image", "logging.volumeStorage") {
		//Clear previous env file
		paramsPath := filepath.Join(localManifestsCopyPath, "vars", "logging-config-map.env")
		if _, err := files.DeleteIfExists(paramsPath); err != nil {
			return "", err
		}
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
	if yamlFile.HasKey("application.defaultNamespace") {
		//Clear previous env file
		paramsPath := filepath.Join(localManifestsCopyPath, "vars", "onepanel-config-map.env")
		if _, err := files.DeleteIfExists(paramsPath); err != nil {
			return "", err
		}
		paramsFile, err := os.Create(paramsPath)
		if err != nil {
			return "", err
		}
		var stringToWrite = fmt.Sprintf("%v=%v\n",
			"applicationDefaultNamespace", flatMap["applicationDefaultNamespace"],
		)
		_, err = paramsFile.WriteString(stringToWrite)
		if err != nil {
			return "", err
		}
	} else {
		log.Fatal("Missing required values in params.yaml, applicationDefaultNamespace")
	}
	//Write to secret files
	var secretKeysValues []string
	artifactRepoSecretPlaceholder := "$(artifactRepositoryProviderSecret)"
	if yamlFile.HasKey("artifactRepository.s3") {
		if yamlFile.HasKeys("artifactRepository.s3.accessKey", "artifactRepository.s3.secretKey") {
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
			log.Fatal("Missing required values in params.yaml, artifactRepository. Check accessKey, or secretKey.")
		}
	}
	if yamlFile.HasKey("artifactRepository.gcs") {
		if yamlFile.HasKey("artifactRepository.gcs.serviceAccountKey") {
			_, val := yamlFile.Get("artifactRepository.gcs.serviceAccountKey")
			if val.Value == "" {
				log.Fatal("artifactRepository.gcs.serviceAccountKey cannot be empty.")
			}
			artifactRepoS3Secret := "artifactRepositoryGCSServiceAccountKey: '" + val.Value + "'"
			err = replacePlaceholderForSecretManiFile(localManifestsCopyPath, artifactRepoSecretPlaceholder, artifactRepoS3Secret)
			if err != nil {
				return "", err
			}
		} else {
			log.Fatal("Missing required values in params.yaml, artifactRepository. artifactRepository.gcs.serviceAccountKey.")
		}
	}

	//To properly replace $(applicationDefaultNamespace), we need to update it in quite a few files.
	//Find those files
	listOfFiles, errorWalking := FilePathWalkDir(localManifestsCopyPath)
	if errorWalking != nil {
		return "", err
	}

	for _, filePath := range listOfFiles {
		manifestFileContent, manifestFileOpenErr := ioutil.ReadFile(filePath)
		if manifestFileOpenErr != nil {
			return "", manifestFileOpenErr
		}
		manifestFileContentStr := string(manifestFileContent)
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
		writeFileErr := ioutil.WriteFile(filePath, []byte(manifestFileContentStr), 0644)
		if writeFileErr != nil {
			return "", writeFileErr
		}
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

func BuilderToTemplate(builder *manifest.Builder) template.Kustomize {
	k := template.Kustomize{
		ApiVersion:     "kustomize.config.k8s.io/v1beta1",
		Kind:           "Kustomization",
		Resources:      make([]string, 0),
		Configurations: []string{filepath.Join("configs/varreference.yaml")},
	}

	for _, overlayComponent := range builder.GetOverlayComponents() {
		if !overlayComponent.HasOverlays() {
			k.Resources = append(k.Resources, overlayComponent.Component().Path())
			continue
		}

		for _, overlay := range overlayComponent.Overlays() {
			k.Resources = append(k.Resources, overlay.Path())
		}
	}

	return k
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

func formatUrlForUi(url string) string {
	result := strings.Replace(url, "/", `\/`, -1)
	result = strings.Replace(result, ".", `\.`, -1)
	result = strings.Replace(result, ":", `\:`, -1)

	return result
}

func runKustomizeBuild(path string) (rm resmap.ResMap, err error) {
	fSys := filesys.MakeFsOnDisk()
	opts := &krusty.Options{
		DoLegacyResourceSort: true,
		LoadRestrictions:     types.LoadRestrictionsNone,
		DoPrune:              false,
	}

	k := krusty.MakeKustomizer(fSys, opts)

	rm, err = k.Run(path)
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
func mapLinkedVars(mapping map[string]interface{}, manifestPath string, config *opConfig.Config) error {
	linkVars := false
	for _, component := range config.Spec.Components {
		if strings.Contains(component, "modeldb") {
			linkVars = true
			break
		}
	}

	if !linkVars {
		return nil
	}

	modelDBMapping, err := util.LoadDynamicYamlFromFile(filepath.Join(manifestPath, "modeldb", "base", "default-vars.yaml"))
	if err != nil {
		return err
	}

	flatMappedVars := modelDBMapping.Flatten(util.LowerCamelCaseFlatMapKeyFormatter)
	for key, valueNode := range flatMappedVars {
		// Skip if key already exists
		if _, ok := mapping[key]; ok {
			continue
		}

		valueKey := util.LowerCamelCaseStringFormat(valueNode.Value.Value, ".")
		value, ok := mapping[valueKey]
		if !ok {
			return fmt.Errorf("unknown key %v", valueKey)
		}

		mapping[key] = value
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
			return fmt.Sprintf("%s can not be '%s', please enter a namespace", paramsError.Key, *paramsError.Value)
		case "blank":
			return fmt.Sprintf("%s can not be blank, please use a different namespace in your params.yaml", paramsError.Key)
		case "reserved":
			return fmt.Sprintf("%s can not be '%v' please use a different namespace in your params.yaml", paramsError.Key, *paramsError.Value)
		}
	}

	return fmt.Sprintf("Error generating result: %v", err.Error())
}
