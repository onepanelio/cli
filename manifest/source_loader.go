package manifest

import (
	"fmt"
	"io/ioutil"
	"os"

	"github.com/onepanelio/cli/config"
	"github.com/onepanelio/cli/files"
	"gopkg.in/yaml.v2"
)

type SourceConfig struct {
	ManifestSourceConfig ManifestSourceConfig `yaml:"manifestSource"`
}

type ManifestSourceConfig struct {
	Github    *GithubSourceConfig    `yaml:"github,omitempty"`
	Directory *DirectorySourceConfig `yaml:"directory,omitempty"`
}

type GithubSourceConfig struct {
	Tag           *string
	OverrideCache *bool `yaml:"overrideCache,omitempty"` // default is false
}

type DirectorySourceConfig struct {
	From          string `yaml:"folder"`
	OverrideCache *bool  `yaml:"overrideCache,omitempty"` // default is false
}

// This will override the file that already exists at path
func CreateGithubSourceConfigFile(path string) error {
	_, err := files.DeleteIfExists(path)
	if err != nil {
		return err
	}

	tag := config.ManifesRepositoryTag

	sourceConfig := SourceConfig{
		ManifestSourceConfig: ManifestSourceConfig{
			Github: &GithubSourceConfig{
				Tag:           &tag,
				OverrideCache: nil,
			},
		},
	}

	data, err := yaml.Marshal(sourceConfig)

	file, err := os.Create(path)
	if err != nil {
		return err
	}

	_, err = file.Write(data)

	return err
}

// Loads and creates the manifest directory in the toPath directory from a config file, configFilePath.
func LoadManifestSourceFromFileConfig(configFilePath string) (source Source, err error) {
	exists, err := files.Exists(configFilePath)
	if err != nil {
		return
	}

	if !exists {
		return nil, fmt.Errorf("unable to load source from config file. File %v does not exist", configFilePath)
	}

	config := &SourceConfig{}
	fileData, err := ioutil.ReadFile(configFilePath)
	if err != nil {
		return
	}

	if err := yaml.Unmarshal(fileData, config); err != nil {
		return nil, err
	}

	if config.ManifestSourceConfig.Github != nil {
		return loadGithubSource(config.ManifestSourceConfig.Github)
	}

	if config.ManifestSourceConfig.Directory != nil {
		return loadDirectorySource(config.ManifestSourceConfig.Directory)
	}

	return nil, fmt.Errorf("%v is badly formatted. No Source Config found", configFilePath)
}

func loadGithubSource(config *GithubSourceConfig) (source Source, err error) {
	if config.Tag == nil {
		latest := "latest"
		config.Tag = &latest
	}

	if config.OverrideCache == nil {
		overrideCache := false
		config.OverrideCache = &overrideCache
	}

	return CreateGithubSource(*config.Tag, *config.OverrideCache)
}

func loadDirectorySource(config *DirectorySourceConfig) (source Source, err error) {
	if config.OverrideCache == nil {
		overrideCache := false
		config.OverrideCache = &overrideCache
	}

	return CreateDirectorySource(config.From, *config.OverrideCache)
}
