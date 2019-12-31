package config

import (
	"fmt"
	"github.com/ghodss/yaml"
	"github.com/onepanelio/cli/files"
	"io/ioutil"
	"strings"
)

type SimpleOverlayedComponent struct {
	parts []*string
}

func CreateSimpleOverlayedComponent(names ...string) *SimpleOverlayedComponent {
	newItem := &SimpleOverlayedComponent{
		parts: make([]*string, 0),
	}

	for _, name := range names {
		newItem.AddPart(&name)
	}

	return newItem
}

func (s *SimpleOverlayedComponent) AddPart(name *string) {
	s.parts = append(s.parts, name)
}

// If there is one part, return just that part.
// If there is more than one, return all but the first.
func (s *SimpleOverlayedComponent) PartsSkipFirst() []*string {
	if len(s.parts) == 1 {
		return s.parts
	}

	return s.parts[1:]
}

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

func (c *Config) GetOverlayComponents() []*SimpleOverlayedComponent {
	overlayedComponents := make([]*SimpleOverlayedComponent, 0)

	mappedComponents := make(map[string]*SimpleOverlayedComponent)

	for _, component := range c.Spec.Components {
		formattedName := strings.TrimSuffix(component, "/base")
		mappedComponents[formattedName] = CreateSimpleOverlayedComponent(component)
	}

	for _, overlay := range c.Spec.Overlays {
		overlaysIndex := strings.Index(overlay, "/overlays")
		formattedName := overlay[:overlaysIndex]

		if _, ok := mappedComponents[formattedName]; ok {
			mappedComponents[formattedName].AddPart(&overlay)
		}
	}

	for key := range mappedComponents {
		overlayedComponents = append(overlayedComponents, mappedComponents[key])
	}

	return overlayedComponents
//	TODO
//	Next part is to update the templatization process to use this new array of SimpleOverlayedComponent
// You can even add a method to return an array of all items except the first one - the component.
// make sure the :overlaysIndex is working properly,
// then go ahead and generate the new kustomization file and make sure it all works with minikube.
// no istio.
// Then, maybe delete the old template code, since we don't really need it anymore.
}