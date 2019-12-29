package files

import (
	"github.com/onepanelio/cli/util"
	"log"
)

type ConfigVar struct {
	Name string `yaml:"name"`
	Required bool `yaml:"required"`
	Default *string `yaml:"default"`
}

func (c *ConfigVar) HasDefault() bool {
	return c.Default != nil
}

type ComponentConfigVar struct {
	ComponantPath string
	ConfigVar *ConfigVar
}

type VarsFile struct {
	Vars []*ConfigVar `yaml:"vars"`
}

func CreateVarsFile() *VarsFile {
	file := &VarsFile{
		Vars: make([]*ConfigVar, 0),
	}

	return file
}

// Given the .env file at path (assumed to exist)
// read through, and add any variables that are not in newVars with a value of TODO
// e.g.
// email=TODO

func MergeParametersFiles(path string, newVars []*ComponentConfigVar) (result *util.DynamicYaml, err error) {
	yamlFile, err := util.LoadDynamicYaml(path)
	if err != nil {
		return nil, err
	}


	// TODO how do you tell if it exsits?
	for _, newVar := range newVars {
		value := yamlFile.Get(newVar.ComponantPath)
		if value == "" {
			// TODO does not exist, add it.
			log.Printf("Does not exist")
		} else
		{
			log.Printf("Exists")
		}
	}

	// Go through each new var.
	// If it already exists in the map, skip it.
	// Otherwise, add it to the map

	awsRegion := yamlFile.Get("cert-manager.aws.region")
	log.Printf("aws region", awsRegion)
	//
	//mappedVars := make(map[string]*ComponentConfigVar)
	//for i := range newVars {
	//	componentConfigVar := newVars[i]
	//	mappedVars[componentConfigVar.ConfigVar.Name] = componentConfigVar
	//}
	//
	//
	//fileString := string(fileData)
	//result = ""
	//
	//fileLines := strings.Split(fileString, "\n")
	//for i := range fileLines {
	//	fileLine := fileLines[i]
	//
	//	// Skip comments
	//	if strings.HasPrefix(fileLine, "#") {
	//		continue
	//	}
	//
	//	envVarParts := strings.Split(fileLine, "=")
	//	if len(envVarParts) > 1 {
	//		varName := envVarParts[0]
	//		if _, ok := mappedVars[varName]; ok {
	//			delete(mappedVars, varName)
	//		}
	//	}
	//
	//	if i == (len(fileLines) - 1) {
	//		result += fileLine
	//	} else {
	//		result += fileLine + "\n"
	//	}
	//}
	//
	//// TODO sort by component and add meta info
	//for key := range mappedVars {
	//	componentConfigVar := mappedVars[key]
	//	if componentConfigVar.ConfigVar.HasDefault() {
	//		result += fmt.Sprintf("\n%v=%v", key, *componentConfigVar.ConfigVar.Default)
	//	} else {
	//		result += fmt.Sprintf("\n%v=%v", key, "TODO")
	//	}
	//}

	return yamlFile, nil
}
