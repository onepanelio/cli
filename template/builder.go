package template

import (
	"github.com/onepanelio/cli/config"
	"github.com/onepanelio/cli/files"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// The source of a configuration file used.
type Source struct {
	AbsolutePath string // path in the filesystem to the file used
	ManifestPath string // path relative to the manifests root directory
	Name string // name of the configuration. E.g. istio, istio-crds. This is a configuration which must have a base directory under it.
	IsOverlay bool
	Order int
}

// A component used for the configuration, such as istio, istio-crds, argo, etc.
type Component struct {
	Name string
}

// An overlay specified for all components. E.g. gcp. If a configuration has a gcp specific overlay
// it should be used.
type Overlay struct {
	Name string
}

type BuilderConfig struct {
	ManifestRoot string // absolute path to the manifests directory containing the kustomize files.
	Components []Component
	Overlays []Overlay
}

type GeneratorItem struct {
	DisableNameSuffixHash bool `yaml:"disableNameSuffixHash"`
}
type ConfigMapItem struct {
	Name string `yaml:"name"`
	Envs []string `yaml:"envs"`
}

type ObjectRef struct {
	Kind string `yaml:"kind"`
	Name string `yaml:"name"`
	ApiVersion string `yaml:"apiVersion"`
}

type FieldRef struct {
	FieldPath string `yaml:"fieldpath"`
}
type VarItem struct {
	Name string `yaml:"name"`
	ObjRef ObjectRef `yaml:"objref"`
	FieldRef FieldRef `yaml:"fieldref"`
}

type Kustomize struct {
	ApiVersion string `yaml:"apiVersion"`
	Kind string `yaml:"kind"`
	Resources []string `yaml:"resources"`
	Configurations []string `yaml:"configurations"`
	ConfigMapItems []ConfigMapItem `yaml:"configMapGenerator"`
	GeneratorOptions GeneratorItem `yaml:"generatorOptions"`
	Vars []VarItem `yaml:"vars"`
}

type Builder struct {
	Sources map[string][]Source
	KeyedComponents map[string]Component
	KeyedOverlays map[string]Overlay
	ManifestRoot string
	Vars map[string]string
}

func createBuilder() Builder {
	return Builder{
		Sources:         make(map[string][]Source),
		KeyedComponents: make(map[string]Component),
		KeyedOverlays:   make(map[string]Overlay),
		Vars: make(map[string]string),
	}
}

func NewBuilder(config BuilderConfig) Builder {
	b := createBuilder()
	b.ManifestRoot = config.ManifestRoot

	for i := range config.Components {
		component := config.Components[i]
		b.KeyedComponents[component.Name] = component
	}

	for i := range config.Overlays {
		overlay := config.Overlays[i]
		b.KeyedOverlays[overlay.Name] = overlay
	}

	return b
}

func NewBuilderFromConfig(config config.Config) Builder {
	b := createBuilder()
	b.ManifestRoot = config.Spec.ManifestsRepo

	for i := range config.Spec.Components {
		component := config.Spec.Components[i]
		b.KeyedComponents[component] = Component{Name:component}
	}

	for i := range config.Spec.Overlays {
		overlay := config.Spec.Overlays[i]
		b.KeyedOverlays[overlay] = Overlay{Name:overlay}
	}

	return b
}

func (b *Builder) Build() error  {
	root := b.ManifestRoot
	commonComponentsRoot := filepath.Join(root, "common")

	err := filepath.Walk(commonComponentsRoot, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if info.Name() == "vars.yaml" {
			if err := b.addVarsFile(path); err != nil {
				return err
			}
		}

		// Don't consider individual files unless its vars.yaml, just overlays and components.
		if !info.IsDir() {
			return nil
		}

		relativePath, relErr := filepath.Rel(commonComponentsRoot, path)
		if relErr != nil {
			log.Printf("Relative Path: err %v", relErr)
			return relErr
		}

		parts := strings.Split(relativePath, string(os.PathSeparator))

		// component/base
		componentName := parts[0]
		if len(parts) == 2 && parts[1] == "base" {
			return b.addComponent(path, componentName, -9999)
		}

		if len(parts) > 2 && parts[1] == "overlays" {
			overlayName := parts[2]
			return b.considerOverlay(path, componentName, overlayName, -9999)
		}

		return nil
	})
	if err != nil {
		return err
	}

	err = filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// skip common directory as we already processed it
		if info.Name() == "common" {
			return filepath.SkipDir
		}

		if info.Name() == "vars.yaml" {
			if err := b.addVarsFile(path); err != nil {
				return err
			}
		}

		// Don't consider individual files unless its vars.yaml, just overlays and components.
		if !info.IsDir() {
			return nil
		}

		relativePath, relErr := filepath.Rel(root, path)
		if relErr != nil {
			log.Printf("Relative Path: err %v", relErr)
			return relErr
		}

		parts := strings.Split(relativePath, string(os.PathSeparator))

		for i, part := range parts {
			if part == "base" && i > 0 {
				componentName := parts[i-1]
				return b.considerComponent(path, componentName, 1)
			}

			if part == "overlays" && i > 0 && i != (len(parts) -1) {
				componentName := parts[i - 1]
				overlayName := parts[i + 1]
				return b.considerOverlay(path, componentName, overlayName, 1)
			}
		}

		return nil
	})

	return err
}

func (b *Builder) Template() Kustomize {
	k := Kustomize{
		ApiVersion: "kustomize.config.k8s.io/v1beta1",
		Kind: "Kustomization",
		Resources: make([]string, 0),
		Configurations: []string{"configs/varreference.yaml"},
	}

	sources := b.flattenSources()

	//put them all in a slice and sort them and append them
	sort.SliceStable(sources, func(i, j int) bool {
		sourceI := sources[i]
		sourceJ := sources[j]

		return sourceI.Order < sourceJ.Order
	})

	for _, source := range sources {
		k.Resources = append(k.Resources, source.ManifestPath)
	}

	return k
}

func (b *Builder) flattenSources() []Source {
	sources := make([]Source, 0)

	for key := range b.Sources {
		for i := range b.Sources[key] {
			source := b.Sources[key][i]
			sources = append(sources, source)
		}
	}

	return sources
}

func (b *Builder) addComponent(path, componentName string, order int) error {
	b.KeyedComponents[componentName] = Component{
		Name:componentName,
	}

	return b.addOrReplaceSource(path, componentName, order, false)
}

func (b *Builder) considerComponent(path, componentName string, order int) error {
	if _, ok := b.KeyedComponents[componentName]; !ok {
		return nil
	}

	return b.addOrReplaceSource(path, componentName, order,false)
}

// Given an Overlay, checks if the manifest uses it and replaces the component if it does.
// If the overlay is not used in the manifest, it is ignored.
func (b *Builder) considerOverlay(path, componentName, overlayName string, order int) error {
	relativePath, err := filepath.Rel(b.ManifestRoot, path)
	if err != nil {
		return err
	}

	if _, ok := b.KeyedOverlays[relativePath]; !ok {
		return nil
	}

	// Don't do overlays for components we don't pick
	if _, ok := b.KeyedComponents[componentName]; !ok {
		return nil
	}

	return b.addOrReplaceSource(path, componentName, order, true)
}

func (b* Builder) addOrReplaceSource(path, componentName string, order int, isOverlay bool) error {
	manifestPath, err := filepath.Rel(b.ManifestRoot, path)
	if err != nil {
		return err
	}

	if _, ok := b.Sources[componentName]; !ok {
		b.Sources[componentName] = make([]Source, 0)
	}

	newSource := Source{
		AbsolutePath: path,
		ManifestPath: manifestPath,
		Name:         componentName,
		IsOverlay: isOverlay,
		Order: order,
	}

	if !isOverlay || len(b.Sources[componentName]) == 0 {
		b.Sources[componentName] = append(b.Sources[componentName], newSource)
		return nil
	}

	for i := range b.Sources[componentName] {
		source := b.Sources[componentName][i]

		if !source.IsOverlay {
			b.Sources[componentName][i] = newSource
			return nil
		}
	}

	b.Sources[componentName] = append(b.Sources[componentName], newSource)

	return nil
}


// TODO remove?
func (b *Builder) addVarsFile(path string) error {
	// TODO skip env vars if the component or overlay is part of the ones considered
	//data, err := ioutil.ReadFile(path)
	//if err != nil {
	//	return err
	//}

	//varFile := files.CreateVarsFile()
	//if err := yaml.Unmarshal(data, &varFile); err != nil {
	//	return err
	//}

	// TODO
	//for i := range varFile.Vars {
	//	varName := varFile.Vars[i]
	//	b.addVar(path, varName)
	//}

	return nil
}

// Adds a variable from the configuration.
// We need the path of the file and the name of the variable.
func (b* Builder) addVar(path string, value *files.ConfigVar) {
	// TODO remove?
	b.Vars[path] = path
}

// Returns an array containing the var names
func (b *Builder) VarsArray() []string {
	result := make([]string, 0)

	for key := range b.Vars {
		result = append(result, key)
	}

	return result
}