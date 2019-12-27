package config

import (
	"fmt"
	"github.com/ghodss/yaml"
	"github.com/onepanelio/cli/files"
	"io/ioutil"
)

type Config struct {
	ApiVersion string `yaml:"apiVersion"`
	Kind string `yaml:"kind"`
	Spec ConfigSpec `yaml:"spec"`
}

type ConfigSpec struct {
	ManifestsRepo string `yaml:"manifestsRepo"`
	Params string `yaml:"params"`
	Components []string `yaml:"components"`
	Overlays []string `yaml:"overlays"`
}

func FromFile(path string) (config *Config, err error) {
	content, err := ioutil.ReadFile(path)
	if err != nil {
		return
	}

	config = &Config{}
	err = yaml.Unmarshal(content, config)
	if err != nil {
		return
	}

	err = config.Validate()

	return
}

// Checks the config to make sure all the set files exist, etc.
// Errors are returned in a human friendly format, and can be printed to stdout.
func (c *Config) Validate() error {
	manifestsExists, err := files.Exists(c.Spec.ManifestsRepo)
	if err != nil {
		return fmt.Errorf("unable to check if file exists at %v", c.Spec.ManifestsRepo)
	}
	if !manifestsExists {
		return fmt.Errorf("the manifests repo directory does not exist at %v", c.Spec.ManifestsRepo)
	}

	paramsExists, err := files.Exists(c.Spec.Params)
	if err != nil {
		return fmt.Errorf("unable to check if file exists at %v", c.Spec.Params)
	}
	if !paramsExists {
		return fmt.Errorf("configuration file error: the parameters file does not exist at %v", c.Spec.Params)
	}

	return nil
}

func (c *Config) AddComponent(name string) {
	c.Spec.Components = append(c.Spec.Components, name)
}

func (c *Config) AddOverlay(name string) {
	c.Spec.Overlays = append(c.Spec.Overlays, name)
}