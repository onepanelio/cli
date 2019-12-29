package manifest

import (
	"fmt"
	"github.com/onepanelio/cli/files"
	"gopkg.in/yaml.v2"
	"io/ioutil"
	"log"
	"os"
	"strings"
)

type OverlayedComponent struct {
	component *Component
	overlays []*Overlay
}

func (c *OverlayedComponent) Component() *Component {
	return c.component
}

func (c *OverlayedComponent) Overlays() []*Overlay {
	return c.overlays
}

func (c *OverlayedComponent) AddOverlay(overlay *Overlay) {
	c.overlays = append(c.overlays, overlay)
}

type Builder struct {
	manifest *Manifest
	overlayedComponents map[string]*OverlayedComponent
}

func CreateBuilder(manifest *Manifest) *Builder {
	b := &Builder{
		manifest: manifest,
		overlayedComponents: make(map[string]*OverlayedComponent),
	}

	return b
}

func (b *Builder) AddComponent(componentPath string) error {
	component := b.manifest.GetComponent(componentPath)

	if component == nil {
		return fmt.Errorf("unknown component '%v'", componentPath)
	}

	if _, ok := b.overlayedComponents[componentPath]; ok {
		return fmt.Errorf("component '%v' has already been added", componentPath)
	}

	overlayedComponent := &OverlayedComponent{
		component: component,
		overlays:  make([]*Overlay, 0),
	}

	b.overlayedComponents[componentPath] = overlayedComponent

	return nil
}

func (b *Builder) AddOverlay(overlayPath string) error {
	overlay := b.manifest.GetOverlay(overlayPath)

	if overlay == nil {
		return fmt.Errorf("unknown overlay '%v'", overlayPath)
	}

	componentPath := overlay.component.path

	if _, ok := b.overlayedComponents[componentPath]; !ok {
		if err := b.AddComponent(componentPath); err != nil {
			return err
		}
	}

	b.overlayedComponents[componentPath].AddOverlay(overlay)

	return nil
}

func (b *Builder) AddCommonComponents() error {
	for key := range b.manifest.components {
		component := b.manifest.components[key]

		if component.IsCommon() {
			if err := b.AddComponent(component.path); err != nil {
				return err
			}
		}
	}

	return nil
}

func (b *Builder) GetOverlayComponents() []*OverlayedComponent {
	result := make([]*OverlayedComponent, 0)

	for key := range b.overlayedComponents {
		result = append(result, b.overlayedComponents[key])
	}

	return result
}


func (b *Builder) GetVarsArray() []*files.ComponentConfigVar {
	varsArray := make([]*files.ComponentConfigVar, 0)

	filePaths := b.GetVarsFilePaths()

	for _, path := range filePaths {
		contents, err := ioutil.ReadFile(path)
		if err != nil {
			log.Printf("[error] ioutil.ReadFile(%v): %v", path, err.Error())
			continue
		}

		varFile := files.CreateVarsFile()

		tempMap := make(map[string](map[string]*files.ConfigVar))

		if err := yaml.Unmarshal(contents, tempMap); err != nil {
			log.Printf("[error] yaml.Unmarshal file %v. Error %v", path, err.Error())
			continue
		}

		for _, item := range varFile.Vars {
			formattedPath := strings.TrimPrefix(path, b.manifest.path + "/")
			formattedPath = strings.TrimSuffix(formattedPath, "/vars.yaml")

			componentConfigVar := &files.ComponentConfigVar{
				ComponantPath: formattedPath,
				ConfigVar:     item,
			}
			varsArray = append(varsArray, componentConfigVar)
		}
	}

	return varsArray
}

// Gets all of the existing vars file paths.
func (b *Builder) GetVarsFilePaths() []string {
	vars := make([]string, 0)

	for key := range b.overlayedComponents {
		overlayComponent := b.overlayedComponents[key]
		vars = append(vars, overlayComponent.component.VarsFilePath())

		for _, overlay := range overlayComponent.Overlays() {
			vars = append(vars, overlay.VarsFilePath())
		}
	}

	existingFilePaths := make([]string, 0)

	for _, path := range vars {
		fullPath := b.manifest.path + string(os.PathSeparator) + path
		exists, err := files.Exists(fullPath)
		if err != nil {
			log.Printf("[error] files.Exists(%v) %v", path, err.Error())
			continue
		}
		if !exists {
			continue
		}

		existingFilePaths = append(existingFilePaths, fullPath)
	}

	return existingFilePaths
}
