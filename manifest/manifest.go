package manifest

import (
	"fmt"
	"github.com/onepanelio/cli/util"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

type Manifest struct {
	path       string // where the manifest directory is located
	components map[string]*Component
	overlays   map[string]*Overlay
}

// ParamsError represents an error encountered in the params.yaml file
type ParamsError struct {
	Key               string // The full key, as in 'application.domain'
	ShortKey          string // The short key, as in 'domain'
	Value             *string
	ErrorType         string
	ValidationMessage string // empty space means none
}

// Error returns an error string indicating what key/value is invalid
func (p *ParamsError) Error() string {
	if p.Value == nil {
		return fmt.Sprintf("%s is invalid", p.Key)
	}

	return fmt.Sprintf("%s: %s is invalid", p.Key, *p.Value)
}

func LoadManifest(manifestRoot string) (*Manifest, error) {
	m := &Manifest{
		path:       manifestRoot,
		components: make(map[string]*Component),
		overlays:   make(map[string]*Overlay),
	}

	err := filepath.Walk(manifestRoot, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Don't consider individual files
		if !info.IsDir() {
			return nil
		}

		relativePath, relErr := filepath.Rel(manifestRoot, path)
		if relErr != nil {
			log.Printf("Relative Path: err %v", relErr)
			return relErr
		}

		parts := strings.Split(relativePath, string(os.PathSeparator))

		for i, part := range parts {
			if part == "base" {
				pathUpToBase := strings.Join(parts[:i], string(os.PathSeparator))
				m.addComponent(pathUpToBase)
			}

			if i > 0 && parts[i-1] == "overlays" {
				pathUpToOverlay := strings.Join(parts[:i+1], string(os.PathSeparator))
				m.addOverlay(pathUpToOverlay)
			}
		}

		return nil
	})

	return m, err
}

// relative path: something (part of something/base)
func (m *Manifest) addComponent(relativePath string) *Component {
	component, ok := m.components[relativePath]
	if ok {
		return component
	}

	newComponent := CreateComponent(relativePath)
	m.components[relativePath] = newComponent

	return newComponent
}

// relative path: something/overlays/something2
func (m *Manifest) addOverlay(relativePath string) *Overlay {
	overlay, ok := m.overlays[relativePath]
	if ok {
		return overlay
	}

	startOfOverlaysIndex := strings.LastIndex(relativePath, "overlays")
	componentPath := relativePath[:startOfOverlaysIndex-1]

	component := m.addComponent(componentPath)

	overlay = CreateOverlay(relativePath, component)
	component.AddOverlay(overlay)

	m.overlays[relativePath] = overlay

	return overlay
}

func (m *Manifest) GetComponent(path string) *Component {
	return m.components[path]
}

func (m *Manifest) GetOverlay(path string) *Overlay {
	return m.overlays[path]
}

// Validate checks if the manifest is valid. If it is, nil is returned. Otherwise an error is returned.
func Validate(manifest *util.DynamicYaml) error {
	reservedNamespaces := map[string]bool{
		"onepanel":           true,
		"application-system": true,
		"cert-manager":       true,
		"istio-system":       true,
		"knative-serving":    true,
		"kube-public":        true,
		"kube-system":        true,
		"default":            true,
	}

	defaultNamespace := manifest.GetValue("application.defaultNamespace")
	if defaultNamespace == nil {
		return &ParamsError{Key: "application.defaultNamespace", ShortKey: "defaultNamespace", ErrorType: "missing"}
	}
	if defaultNamespace.Value == "" {
		return &ParamsError{Key: "application.defaultNamespace", ShortKey: "defaultNamespace", ErrorType: "blank"}
	}
	if defaultNamespace.Value == "<namespace>" {
		return &ParamsError{
			Key:               "application.defaultNamespace",
			ShortKey:          "defaultNamespace",
			Value:             &defaultNamespace.Value,
			ErrorType:         "parameter",
			ValidationMessage: "Namespace can not be <namespace> please provide a value like 'example'",
		}
	}
	if _, ok := reservedNamespaces[defaultNamespace.Value]; ok {
		return &ParamsError{Key: "application.defaultNamespace", ShortKey: "defaultNamespace", Value: &defaultNamespace.Value, ErrorType: "reserved"}
	}

	if len(defaultNamespace.Value) > 63 {
		return &ParamsError{
			Key:               "application.defaultNamespace",
			ShortKey:          "defaultNamespace",
			Value:             &defaultNamespace.Value,
			ErrorType:         "parameter",
			ValidationMessage: "Namespace must be less than 63 characters",
		}
	}

	if strings.HasPrefix(defaultNamespace.Value, "kube-") {
		return &ParamsError{
			Key:               "application.defaultNamespace",
			ShortKey:          "defaultNamespace",
			Value:             &defaultNamespace.Value,
			ErrorType:         "parameter",
			ValidationMessage: "A namespace can not start with 'kube-'",
		}
	}

	namespaceRegex := regexp.MustCompile(`^[a-z0-9]([-a-z0-9]*[a-z0-9])$`)
	if !namespaceRegex.MatchString(defaultNamespace.Value) {
		return &ParamsError{
			Key:               "application.defaultNamespace",
			ShortKey:          "defaultNamespace",
			Value:             &defaultNamespace.Value,
			ErrorType:         "parameter",
			ValidationMessage: "A namespace can not start with 'kube-', must be lowercase, and can not start or end with dashes '-'",
		}
	}

	domain := manifest.GetValue("application.domain")
	if domain == nil {
		return &ParamsError{Key: "application.domain", ShortKey: "domain", ErrorType: "missing"}
	}
	if domain.Value == "" {
		return &ParamsError{Key: "application.domain", ShortKey: "domain", ErrorType: "blank"}
	}

	fqdn := manifest.GetValue("application.fqdn")
	if fqdn == nil {
		return &ParamsError{Key: "application.fqdn", ShortKey: "fqdn", ErrorType: "missing"}
	}
	if fqdn.Value == "" {
		return &ParamsError{Key: "application.fqdn", ShortKey: "fqdn", ErrorType: "blank"}
	}

	if !strings.HasSuffix(fqdn.Value, domain.Value) {
		return fmt.Errorf("application.fqdn does not end in application.domain")
	}

	flatMap := manifest.FlattenToKeyValue(util.AppendDotFlatMapKeyFormatter)
	mapKeys := []string{}
	for key := range flatMap {
		mapKeys = append(mapKeys, key)
	}
	sort.Strings(mapKeys)

	for _, key := range mapKeys {
		value := flatMap[key]
		valueString, ok := value.(string)
		if !ok {
			continue
		}

		if strings.HasPrefix(valueString, "<") {
			lastDotIndex := strings.LastIndex(key, ".")
			shortKey := key[lastDotIndex+1:]
			return &ParamsError{Key: key, ShortKey: shortKey, Value: &valueString, ErrorType: "parameter"}
		}
	}

	return nil
}
