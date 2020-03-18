package manifest

import (
	"fmt"
	"github.com/onepanelio/cli/files"
	"github.com/onepanelio/cli/util"
	"log"
	"os"
	"strings"
)

type OverlayedComponent struct {
	component *Component
	overlays  []*Overlay
}

func (c *OverlayedComponent) Component() *Component {
	return c.component
}

func (c *OverlayedComponent) Overlays() []*Overlay {
	return c.overlays
}

func (c *OverlayedComponent) AddOverlay(overlay *Overlay) {

	// Don't add the same overlay twice.
	for _, existingOverlay := range c.overlays {
		if overlay.path == existingOverlay.path {
			return
		}
	}

	c.overlays = append(c.overlays, overlay)
}

func (c *OverlayedComponent) HasOverlays() bool {
	return len(c.overlays) != 0
}

type Builder struct {
	manifest            *Manifest
	overlayedComponents map[string]*OverlayedComponent
	overlayContenders   []string
}

func CreateBuilder(manifest *Manifest) *Builder {
	b := &Builder{
		manifest:            manifest,
		overlayedComponents: make(map[string]*OverlayedComponent),
		overlayContenders:   make([]string, 0),
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

func (b *Builder) AddCommonComponents(skipComponents ...string) error {
	skipMap := make(map[string]bool)
	for _, skip := range skipComponents {
		skipMap[skip] = true
	}

	for key := range b.manifest.components {
		if _, ok := skipMap[key]; ok {
			continue
		}

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

func (b *Builder) AddOverlayContender(contender string) {
	b.overlayContenders = append(b.overlayContenders, contender)
}

func (b *Builder) Build() error {
	// Go through each overlay contender and component, and add the overlays
	for _, overlayContender := range b.overlayContenders {
		for key := range b.manifest.overlays {
			overlay := b.manifest.overlays[key]
			if _, ok := b.overlayedComponents[overlay.component.path]; !ok {
				continue
			}

			if strings.HasSuffix(overlay.path, overlayContender) {
				if err := b.AddOverlay(overlay.path); err != nil {
					return err
				}
			}
		}
	}

	return nil
}

func (b *Builder) GetYamls() []*util.DynamicYaml {
	varsArray := make([]*util.DynamicYaml, 0)

	filePaths := b.GetVarsFilePaths()

	for _, path := range filePaths {
		temp, err := util.LoadDynamicYamlFromFile(path)
		if err != nil {
			log.Printf("[error] LoadDynamicYaml %v. Error %v", path, err.Error())
			continue
		}

		temp.FlattenRequiredDefault()

		varsArray = append(varsArray, temp)
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
