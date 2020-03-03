package cmd

import (
	"fmt"
	opConfig "github.com/onepanelio/cli/config"
	"github.com/onepanelio/cli/files"
	"github.com/onepanelio/cli/manifest"
	"github.com/onepanelio/cli/template"
	"github.com/onepanelio/cli/util"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v2"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

// generateCmd represents the generate command
var generateCmd = &cobra.Command{
	Use:   "generate",
	Short: "Generates a kubernetes yaml configuration file.",
	Long: `Generates a kubernetes yaml configuration file given the 
OpDef file and parameters file, where you can customize components and overlays.

A sample usage is:

op-cli generate config.yaml params.yaml
`,
	Run: func(cmd *cobra.Command, args []string) {
		configFilePath := "config.yaml"

		if len(args) > 1 {
			configFilePath = args[0]
			return
		}

		config, err := opConfig.FromFile(configFilePath)
		if err != nil {
			fmt.Printf("Unable to read configuration file: %v", err.Error())
			return
		}

		kustomizeTemplate := TemplateFromSimpleOverlayedComponents(config.GetOverlayComponents(""))

		result, err := GenerateKustomizeResult(*config, kustomizeTemplate)
		if err != nil {
			log.Printf("Error generating result %v", err.Error())
			return
		}

		fmt.Printf("%v", result)
	},
}

func init() {
	rootCmd.AddCommand(generateCmd)
}

// Given the path to the manifests, and a kustomize config, creates the final kustomization file.
// It does this by copying the manifests into a temporary directory, inserting the kustomize template
// and running the kustomize command
func GenerateKustomizeResult(config opConfig.Config, kustomizeTemplate template.Kustomize) (string, error) {
	manifestPath := config.Spec.ManifestsRepo
	localManifestsCopyPath := ".manifests/cache"

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

	yamlFile, err := util.LoadDynamicYaml(config.Spec.Params)
	if err != nil {
		return "", err
	}

	flatMap := yamlFile.Flatten(util.LowerCamelCaseFlatMapKeyFormatter)
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
		flatMap[keyValArr[0]] = keyValArr[1]
	}

	//Write to env files
	//workflow-config-map.env
	if yamlFile.Get("artifactRepository.bucket") != nil &&
		yamlFile.Get("artifactRepository.endpoint") != nil &&
		yamlFile.Get("artifactRepository.insecure") != nil &&
		yamlFile.Get("artifactRepository.region") != nil {
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
			"artifactRepositoryBucket", flatMap["artifactRepositoryBucket"],
			"artifactRepositoryEndpoint", flatMap["artifactRepositoryEndpoint"],
			"artifactRepositoryInsecure", flatMap["artifactRepositoryInsecure"],
			"artifactRepositoryRegion", flatMap["artifactRepositoryRegion"],
		)
		_, err = paramsFile.WriteString(stringToWrite)
		if err != nil {
			return "", err
		}
	} else {
		log.Fatal("Missing required values in params.yaml, artifactRepository. Check bucket, endpoint, or insecure.")
	}
	//logging-config-map.env, optional component
	if yamlFile.Get("logging.image") != nil &&
		yamlFile.Get("logging.volumeStorage") != nil {
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
	if yamlFile.Get("defaultNamespace") != nil {
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
			"defaultNamespace", flatMap["defaultNamespace"],
		)
		_, err = paramsFile.WriteString(stringToWrite)
		if err != nil {
			return "", err
		}
	} else {
		log.Fatal("Missing required values in params.yaml, defaultNamespace")
	}
	//Write to secret files
	//common/onepanel/base/secrets.yaml
	var secretKeysValues []string
	if yamlFile.Get("artifactRepository.accessKey") != nil &&
		yamlFile.Get("artifactRepository.secretKey") != nil {
		secretKeysValues = append(secretKeysValues, "artifactRepositoryAccessKey", "artifactRepositorySecretKey")
		for _, key := range secretKeysValues {
			//Path to secrets file
			secretsPath := filepath.Join(localManifestsCopyPath, "common", "onepanel", "base", "secret-onepanel-defaultnamespace.yaml")
			//Read the file, replace the specific value, write the file back
			secretFileContent, secretFileOpenErr := ioutil.ReadFile(secretsPath)
			if secretFileOpenErr != nil {
				return "", secretFileOpenErr
			}
			secretFileContentStr := string(secretFileContent)
			value := flatMap[key]
			oldString := "$(" + key + ")"
			if strings.Contains(secretFileContentStr, key) {
				valueStr, ok := value.(string)
				if !ok {
					valueBool, _ := value.(bool)
					valueStr = strconv.FormatBool(valueBool)
				}
				secretFileContentStr = strings.Replace(secretFileContentStr, oldString, valueStr, 1)
				writeFileErr := ioutil.WriteFile(secretsPath, []byte(secretFileContentStr), 0644)
				if writeFileErr != nil {
					return "", writeFileErr
				}
			} else {
				fmt.Printf("Key: %v not present in %v, not used.\n", key, secretsPath)
			}
		}
	} else {
		log.Fatal("Missing required values in params.yaml, artifactRepository. Check accessKey, or secretKey.")
	}

	//To properly replace $(defaultNamespace), we need to update it in quite a few files.
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
		//"defaultNamespace",flatMap["defaultNamespace"]
		configMapCheck := "kind: ConfigMap"
		configMapFile := false
		if strings.Contains(manifestFileContentStr, configMapCheck) {
			configMapFile = true
		}
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
				if configMapFile && useStr == "false" {
					if !strings.Contains(manifestFileContentStr, "config: |") {
						useStr = "\"false\""
					}
				}
				manifestFileContentStr = strings.Replace(manifestFileContentStr, oldString, useStr, -1)
			}
			oldRawString := "$raw(" + key + ")"
			if strings.Contains(manifestFileContentStr, key) {
				manifestFileContentStr = strings.Replace(manifestFileContentStr, oldRawString, rawStr, -1)
			}
		}
		writeFileErr := ioutil.WriteFile(filePath, []byte(manifestFileContentStr), 0644)
		if writeFileErr != nil {
			return "", writeFileErr
		}
		//reset for next file
		configMapFile = false
	}

	//Update the values in those files

	cmd := exec.Command("kustomize", "build", localManifestsCopyPath, "--load_restrictor", "none")
	stdOut, err := cmd.StdoutPipe()
	if err != nil {
		return "", err
	}

	stdErr, err := cmd.StderrPipe()
	if err != nil {
		return "", err
	}

	if err := cmd.Start(); err != nil {
		return "", err
	}

	result, err := ioutil.ReadAll(stdOut)
	if err != nil {
		return "", err
	}

	errRes, err := ioutil.ReadAll(stdErr)
	if err != nil {
		log.Printf("Errors:\n%v", string(errRes))
		return "", err
	}

	if err := cmd.Wait(); err != nil {
		return "", err
	}

	return string(result), nil
}

func BuilderToTemplate(builder *manifest.Builder) template.Kustomize {
	k := template.Kustomize{
		ApiVersion:     "kustomize.config.k8s.io/v1beta1",
		Kind:           "Kustomization",
		Resources:      make([]string, 0),
		Configurations: []string{"configs/varreference.yaml"},
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
		Configurations: []string{"configs/varreference.yaml"},
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
