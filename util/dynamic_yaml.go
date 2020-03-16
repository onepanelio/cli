package util

import (
	"fmt"
	"github.com/iancoleman/strcase"
	"gopkg.in/yaml.v3"
	"io/ioutil"
	"log"
	"strconv"
	"strings"
)

// Path might be an empty string, denoting the start of a path.
// In that case, it is expected that newPart in some formatted form will be return.
// e.g. path = "", newPart = "root". Return: "root"
//      path = "root", newPart = "child". Return: "root.child"
type FlatMapKeyFormatter func(path, newPart string) string

// e.g. path = "", newPart = "root". Return: "root"
//      path = "root", newPart = "child". Return: "root.child"
func AppendDotFlatMapKeyFormatter(path, newPart string) string {
	if path == "" {
		return newPart
	}

	return path + "." + newPart
}

func LowerCamelCaseFlatMapKeyFormatter(path, newPart string) string {
	if path == "" {
		return strcase.ToLowerCamel(newPart)
	}

	return path + strcase.ToCamel(newPart)
}

func CapitalizeUnderscoreFlatMapKeyFormatter(path, newPart string) string {
	if path == "" {
		return strings.ToUpper(newPart)
	}

	return path + "_" + strings.ToUpper(newPart)
}

type DynamicYaml struct {
	node *yaml.Node
}

func LoadDynamicYamlFromFile(filePath string) (*DynamicYaml, error) {
	rawFileData, err := ioutil.ReadFile(filePath)
	if err != nil {
		return nil, err
	}

	data := &yaml.Node{
		Kind:  yaml.DocumentNode,
		Style: 0,
	}
	if err := yaml.Unmarshal(rawFileData, data); err != nil {
		return nil, err
	}

	dynamicYaml := &DynamicYaml{
		node: data,
	}

	return dynamicYaml, nil
}

func LoadDynamicYamlFromString(input string) (*DynamicYaml, error) {
	data := &yaml.Node{}
	if err := yaml.Unmarshal([]byte(input), data); err != nil {
		return nil, err
	}

	dynamicYaml := &DynamicYaml{
		node: data,
	}

	return dynamicYaml, nil
}

func (d *DynamicYaml) GetByParts(parts ...string) *yaml.Node {
	if len(d.node.Content) == 0 {
		return nil
	}

	parentNode := d.node.Content[0]
	var valueNode *yaml.Node

	for _, part := range parts {
		for childIndex, child := range parentNode.Content {
			if child.Value == part {
				valueIndex := childIndex + 1
				if valueIndex >= len(parentNode.Content) {
					return nil
				}

				valueNode = parentNode.Content[valueIndex]
				if valueNode.Kind == yaml.MappingNode {
					parentNode = valueNode
					valueNode = nil
				} else {
					return valueNode
				}

				break
			}
		}
	}

	return valueNode
}

func (d *DynamicYaml) GetWithSeparator(key, separator string) *yaml.Node {
	parts := strings.Split(key, separator)
	return d.GetByParts(parts...)
}

func (d *DynamicYaml) Get(key string) *yaml.Node {
	return d.GetWithSeparator(key, ".")
}

func (d *DynamicYaml) HasKey(key string) bool {
	return d.Get(key) != nil
}

func createMappingYamlNode() *yaml.Node {
	return &yaml.Node{
		Kind:        yaml.MappingNode,
		Style:       0,
		Tag:         "",
		Value:       "",
		Anchor:      "",
		Alias:       nil,
		Content:     []*yaml.Node{},
		HeadComment: "",
		LineComment: "",
		FootComment: "",
		Line:        0,
		Column:      0,
	}
}

func (d *DynamicYaml) PutByParts(parts []string, value interface{}) *yaml.Node {
	if d.node == nil {
		d.node = &yaml.Node{
			Kind: yaml.DocumentNode,
		}
	}

	if len(d.node.Content) == 0 {
		newNode := createMappingYamlNode()
		d.node.Content = append(d.node.Content, newNode)
	}

	parentNode := d.node.Content[0]
	valueNode := d.node.Content[0]
	for index, part := range parts {
		lastPart := index == len(parts)-1
		// if the key doesn't exist, create it.
		// on the last key, put the value.
		exists := false
		for childIndex, child := range parentNode.Content {
			if child.Value == part {
				exists = true
				valueIndex := childIndex + 1
				if valueIndex >= len(parentNode.Content) {
					newNode := createMappingYamlNode()
					parentNode.Content = append(parentNode.Content, newNode)
				}

				valueNode = parentNode.Content[valueIndex]
				if valueNode.Kind == yaml.MappingNode {
					parentNode = valueNode
				}

				break
			}
		}

		if !exists {
			keyNode := &yaml.Node{
				Kind:  yaml.ScalarNode,
				Value: part,
			}

			if !lastPart {
				newNode := createMappingYamlNode()

				parentNode.Content = append(parentNode.Content, keyNode)
				parentNode.Content = append(parentNode.Content, newNode)

				parentNode = newNode
				valueNode = nil
			} else {
				parentNode.Content = append(parentNode.Content, keyNode)

				valueNode = &yaml.Node{
					Kind:  yaml.ScalarNode,
					Value: fmt.Sprintf("%v", value),
				}

				parentNode.Content = append(parentNode.Content, valueNode)
			}
		}
	}

	if valueNode == nil {
		valueNode = &yaml.Node{
			Kind: yaml.ScalarNode,
		}

		parentNode.Content = append(parentNode.Content, valueNode)
	}

	valueNode.Value = fmt.Sprintf("%v", value)

	return valueNode
}

func (d *DynamicYaml) PutWithSeparator(key string, value interface{}, separator string) *yaml.Node {
	return d.PutByParts(strings.Split(key, separator), value)
}

func (d *DynamicYaml) Put(key string, value interface{}) *yaml.Node {
	return d.PutWithSeparator(key, value, ".")
}

func (d *DynamicYaml) String() (string, error) {
	data, err := yaml.Marshal(d.node)
	if err != nil {
		return "", err
	}

	return string(data), nil
}

func (d *DynamicYaml) DeleteByParts(parts ...string) error {
	if len(d.node.Content) == 0 {
		return nil
	}

	parentNode := d.node.Content[0]
	valueNode := d.node.Content[0]

	path := []*yaml.Node{parentNode}

	for _, part := range parts {
		for childIndex, child := range valueNode.Content {
			if child.Value == part {
				valueIndex := childIndex + 1
				if valueIndex >= len(valueNode.Content) {
					return nil
				}

				parentNode = valueNode
				valueNode = valueNode.Content[valueIndex]

				path = append(path, parentNode)

				break
			}
		}
	}

	lastPart := parts[len(parts)-1]

	keptNodes := []*yaml.Node{}
	skip := len(parentNode.Content)
	for i, node := range parentNode.Content {
		if i == skip {
			continue
		}

		if node.Value == lastPart {
			skip = i + 1
			continue
		}

		keptNodes = append(keptNodes, node)
	}

	parentNode.Content = keptNodes

	return nil
}

func (d *DynamicYaml) DeleteByString(key, separator string) error {
	parts := strings.Split(key, separator)
	return d.DeleteByParts(parts...)
}

func (d *DynamicYaml) Delete(key string) error {
	return d.DeleteByString(key, ".")
}

func (d *DynamicYaml) Flatten(keyFormatter FlatMapKeyFormatter) map[string]interface{} {
	results := make(map[string]interface{})

	flattenMap("", keyFormatter, d.node, results)

	return results
}

func flattenMap(path string, keyFormatter FlatMapKeyFormatter, node *yaml.Node, results map[string]interface{}) {
	for i, childNode := range node.Content {
		// this is a value node
		if childNode.Kind == yaml.ScalarNode && (i%2 == 1) {
			key := node.Content[i-1].Value
			newPath := keyFormatter(path, key)

			finalValue, err := NodeValueToActual(childNode)
			if err != nil {
				log.Printf("[error] converting value to correct type: %v", err.Error())
				continue
			}
			results[newPath] = finalValue
		} else if childNode.Kind == yaml.MappingNode && (i%2 == 1) {
			key := node.Content[i-1].Value
			newPath := keyFormatter(path, key)

			flattenMap(newPath, keyFormatter, childNode, results)
			continue
		} else if childNode.Kind == yaml.MappingNode {
			flattenMap(path, keyFormatter, childNode, results)
			continue
		}
	}
}

func NodeValueToActual(node *yaml.Node) (interface{}, error) {
	value := node.Value

	switch node.Tag {
	case "!!bool":
		return strconv.ParseBool(value)
	case "!!int":
		return strconv.ParseInt(value, 10, 32)
	}

	return value, nil
}
