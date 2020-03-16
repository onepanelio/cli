package files

import (
	"github.com/onepanelio/cli/util"
	"strings"
)

type ConfigVar struct {
	Required bool    `yaml:"required"`
	Default  *string `yaml:"default"`
}

type ManifestVariable struct {
	Key      string
	Required bool         `yaml:"required"`
	Default  *interface{} `yaml:"default"`
}

func (c *ConfigVar) HasDefault() bool {
	return c.Default != nil
}

type ComponentConfigVar struct {
	ComponantPath string
	ConfigVar     *ConfigVar
}

type VarsFile util.DynamicYaml

func (v VarsFile) GetVariables() []*ManifestVariable {
	result := make([]*ManifestVariable, 0)

	dynamicYaml := util.DynamicYaml(v)
	flatMap := dynamicYaml.Flatten(util.AppendDotFlatMapKeyFormatter)

	variableMap := make(map[string]bool)

	for key := range flatMap {
		// Lop off the last period to group the variables
		lastPeriodIndex := strings.LastIndex(key, ".")
		newKey := key[:lastPeriodIndex]

		variableMap[newKey] = true
	}

	for key := range variableMap {
		requiredKey := key + ".required"
		defaultKey := key + ".default"

		newVar := &ManifestVariable{
			Key: key,
		}

		if requiredValue, ok := flatMap[requiredKey]; ok {
			requiredValueBool := requiredValue.(bool)
			newVar.Required = requiredValueBool
		}

		if defaultValue, ok := flatMap[defaultKey]; ok {
			newVar.Default = &defaultValue
		}

		result = append(result, newVar)
	}

	return result
}

// Given the parameters file at path (assumed to exist)
// read through, and add any variables that are not in newVars with a value of TODO
func MergeParametersFiles(path string, newVars []*ManifestVariable) (result *util.DynamicYaml, err error) {
	yamlFile, err := util.LoadDynamicYamlFromFile(path)
	if err != nil {
		return nil, err
	}

	for _, newVar := range newVars {
		value := yamlFile.Get(newVar.Key)
		if value == nil {
			if !newVar.Required && newVar.Default == nil {
				continue
			}

			if newVar.Default != nil {
				yamlFile.Put(newVar.Key, *newVar.Default)
			} else {
				yamlFile.Put(newVar.Key, "TODO")
			}
		}
	}

	return yamlFile, nil
}
