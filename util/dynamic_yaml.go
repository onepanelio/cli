package util

import (
	"fmt"
	"github.com/iancoleman/strcase"
	"gopkg.in/yaml.v3"
	"io/ioutil"
	"log"
	"sort"
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

// LowerCamelCaseStringFormat takes a string, splits it by the separator and joins the pieces
// using LowerCamelCaseFlatMapKeyFormatter
func LowerCamelCaseStringFormat(value, separator string) string {
	result := ""

	parts := strings.Split(value, separator)
	for _, part := range parts {
		result = LowerCamelCaseFlatMapKeyFormatter(result, part)
	}

	return result
}

type NodePair struct {
	Key   *yaml.Node
	Value *yaml.Node
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

	formattedFileData := strings.ReplaceAll(string(rawFileData), "\r", "")
	if err := yaml.Unmarshal([]byte(formattedFileData), data); err != nil {
		return nil, err
	}

	dynamicYaml := &DynamicYaml{
		node: data,
	}

	return dynamicYaml, nil
}

func LoadDynamicYamlFromString(input string) (*DynamicYaml, error) {
	data := &yaml.Node{
		Kind:  yaml.DocumentNode,
		Style: 0,
	}
	if err := yaml.Unmarshal([]byte(input), data); err != nil {
		return nil, err
	}

	dynamicYaml := &DynamicYaml{
		node: data,
	}

	return dynamicYaml, nil
}

// SetTopComment sets the topmost comment in the resulting yaml
func (d *DynamicYaml) SetTopComment(comment string) error {
	if d == nil {
		return nil
	}

	if d.node == nil {
		return fmt.Errorf("DynamicYaml has not been loaded yet")
	}

	d.node.HeadComment = comment

	return nil
}

func (d *DynamicYaml) GetByParts(parts ...string) (key, value *yaml.Node) {
	if len(d.node.Content) == 0 {
		return nil, nil
	}

	parentNode := d.node.Content[0]
	var keyNode *yaml.Node
	var valueNode *yaml.Node

	for _, part := range parts {
		foundKey := false
		for keyIndex := 0; keyIndex < len(parentNode.Content)-1; keyIndex++ {
			keyNode = parentNode.Content[keyIndex]
			valueNode = parentNode.Content[keyIndex+1]

			if keyNode.Value != part {
				continue
			}
			foundKey = true

			if valueNode.Kind == yaml.MappingNode {
				parentNode = valueNode
			}

			// We found the key, so no need to check the other keys
			break
		}

		// We did not find a key in the chain, so no need to look further down the chain.
		if !foundKey {
			break
		}
	}

	lastPart := parts[len(parts)-1]
	if lastPart != keyNode.Value {
		return nil, nil
	}

	key = keyNode
	value = valueNode

	return
}

func (d *DynamicYaml) GetValueByParts(parts ...string) (value *yaml.Node) {
	_, value = d.GetByParts(parts...)

	return
}

func (d *DynamicYaml) GetWithSeparator(key, separator string) (keyNode, value *yaml.Node) {
	parts := strings.Split(key, separator)
	return d.GetByParts(parts...)
}

func (d *DynamicYaml) GetValueWithSeparator(key, separator string) (value *yaml.Node) {
	parts := strings.Split(key, separator)
	return d.GetValueByParts(parts...)
}

func (d *DynamicYaml) Get(key string) (keyNode, value *yaml.Node) {
	return d.GetWithSeparator(key, ".")
}

func (d *DynamicYaml) GetValue(key string) (value *yaml.Node) {
	return d.GetValueWithSeparator(key, ".")
}

func (d *DynamicYaml) HasKey(key string) bool {
	_, value := d.Get(key)

	return value != nil
}

func (d *DynamicYaml) HasKeys(keys ...string) bool {
	for _, key := range keys {
		if !d.HasKey(key) {
			return false
		}
	}

	return true
}

// FindMissingKeys will return an array of the keys that are not in the manifest
func (d *DynamicYaml) FindMissingKeys(keys ...string) []string {
	missing := make([]string, 0)

	for _, key := range keys {
		if !d.HasKey(key) {
			missing = append(missing, key)
		}
	}

	return missing
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

func (d *DynamicYaml) PutByPartsNode(parts []string, value *yaml.Node) (*yaml.Node, error) {
	if value == nil {
		return nil, fmt.Errorf("nil passed in as value")
	}

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
		parentNode.Content = append(parentNode.Content, value)
		valueNode = value
	}

	*valueNode = *value

	return valueNode, nil
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

func (d *DynamicYaml) PutWithSeparatorNode(key string, value *yaml.Node, separator string) (*yaml.Node, error) {
	return d.PutByPartsNode(strings.Split(key, separator), value)
}

func (d *DynamicYaml) Put(key string, value interface{}) *yaml.Node {
	return d.PutWithSeparator(key, value, ".")
}

func (d *DynamicYaml) PutNode(key string, value *yaml.Node) (*yaml.Node, error) {
	return d.PutWithSeparatorNode(key, value, ".")
}

func (d *DynamicYaml) Sort() {
	sortNode(d.node)
}

func sortNode(node *yaml.Node) {
	sortNodePairs(node)

	for _, child := range node.Content {
		if child.Kind == yaml.MappingNode {
			sortNode(child)
		}
	}
}

func sortNodePairs(node *yaml.Node) {
	if node.Kind != yaml.MappingNode {
		return
	}

	keys := make([]*yaml.Node, 0)
	keyChild := make(map[string]*yaml.Node)
	for i := 0; i < len(node.Content)-1; i += 2 {
		keyNode := node.Content[i]
		valueNode := node.Content[i+1]

		keyChild[keyNode.Value] = valueNode
		keys = append(keys, keyNode)
	}

	sort.Slice(keys, func(i, j int) bool {
		keyA := keys[i]
		keyB := keys[j]
		return strings.Compare(keyA.Value, keyB.Value) < 0
	})

	sorted := make([]*yaml.Node, 0)
	for _, key := range keys {
		sorted = append(sorted, key)

		if value, ok := keyChild[key.Value]; ok {
			sorted = append(sorted, value)
		}
	}

	node.Content = sorted
}

func (d *DynamicYaml) String() (string, error) {
	builder := &strings.Builder{}
	encoder := yaml.NewEncoder(builder)
	encoder.SetIndent(2)

	defer encoder.Close()
	err := encoder.Encode(d.node)
	if err != nil {
		return "", err
	}

	data := builder.String()

	return data, nil
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

func (d *DynamicYaml) Flatten(keyFormatter FlatMapKeyFormatter) map[string]NodePair {
	results := make(map[string]NodePair)

	flattenMap("", keyFormatter, d.node, results)

	return results
}

func (d *DynamicYaml) FlattenToKeyValue(keyFormatter FlatMapKeyFormatter) map[string]interface{} {
	results := make(map[string]NodePair)

	flattenMap("", keyFormatter, d.node, results)

	flatResult := make(map[string]interface{})

	for key := range results {
		value, err := NodeValueToActual(results[key].Value)
		if err != nil {
			log.Fatal("Unable to convert node value: %v", err.Error())
			continue
		}

		flatResult[key] = value
	}

	return flatResult
}

// FlattenRequiredDefault goes through the data and finds values with a default
// it then assigns the default value to the key in the data so when it is serialized it has that set.
// this also removes the subdata, so if you have s3.bucket.default, and s3.bucket.test, only s3.bucket will be present after.
func (d *DynamicYaml) FlattenRequiredDefault() {
	flatMap := d.Flatten(AppendDotFlatMapKeyFormatter)

	for key := range flatMap {
		//Handle case of application.defaultNamespace.default
		lastIndex := strings.LastIndex(key, ".")
		if lastIndex < 0 {
			continue
		}
		postfixDefault := ".default"
		if key[lastIndex:] != postfixDefault {
			continue
		}

		defaultIndex := strings.LastIndex(key, postfixDefault)
		if defaultIndex < 0 {
			continue
		}

		partialKey := key[0:defaultIndex]
		_, partialNode := d.Get(key)

		valueNode := d.Put(partialKey, partialNode.Value)
		valueNode.Content = []*yaml.Node{}
		valueNode.Kind = partialNode.Kind
		valueNode.Tag = partialNode.Tag
	}
}

func (d *DynamicYaml) mergeSingle(y *DynamicYaml) {
	if len(y.node.Content) == 0 || len(y.node.Content[0].Content) == 0 {
		return
	}

	if d.node == nil {
		d.node = &yaml.Node{
			Kind: yaml.DocumentNode,
		}
	}

	if len(d.node.Content) == 0 {
		newNode := createMappingYamlNode()
		d.node.Content = append(d.node.Content, newNode)
	}

	destination := d.node.Content[0]
	values := y.node.Content[0]
	for i := 0; i < len(values.Content)-1; i += 2 {
		keyNode := values.Content[i]
		valueNode := values.Content[i+1]

		alreadyExists := false
		var jKey *yaml.Node
		var jValue *yaml.Node
		for j := 0; j < len(destination.Content)-1; j++ {
			jKey = destination.Content[j]
			jValue = destination.Content[j+1]

			if keyNode.Value == jKey.Value {
				alreadyExists = true
				break
			}
		}

		if alreadyExists {
			if jKey != nil {
				// Always replace the comments with the new comment values
				// This makes it less generic, but it makes the source YAML comments
				// always overwrite any custom yaml, which is the expected behavior.
				jKey.HeadComment = keyNode.HeadComment
				jKey.LineComment = keyNode.LineComment
				jKey.FootComment = keyNode.FootComment
			}
			mergeNodes(jValue, valueNode)
		} else {
			destination.Content = append(destination.Content, keyNode)
			destination.Content = append(destination.Content, valueNode)
		}
	}
}

// Merge will merge two DynamicYaml's together. If keys already exist, the source is kept.
func (d *DynamicYaml) Merge(items ...*DynamicYaml) {
	for _, item := range items {
		d.mergeSingle(item)
	}
}

func mergeNodes(a, b *yaml.Node) {
	// We can't do anything if it's just a key.
	if a.Kind == yaml.ScalarNode {
		return
	}

	if a.Kind == yaml.MappingNode && b.Kind == yaml.MappingNode {
		for i := 0; i < len(b.Content)-1; i += 2 {
			bKeyNode := b.Content[i]
			bValueNode := b.Content[i+1]

			alreadyExists := false
			var aKeyNode *yaml.Node = nil
			for j := 0; j < len(a.Content)-1; j += 2 {
				aKeyNode = a.Content[j]
				aValueNode := a.Content[j+1]

				if aKeyNode.Value == bKeyNode.Value {
					alreadyExists = true

					mergeNodes(aValueNode, bValueNode)

					break
				}
			}

			if !alreadyExists {
				a.Content = append(a.Content, bKeyNode)
				a.Content = append(a.Content, bValueNode)
			}
		}
	}
}

func flattenMap(path string, keyFormatter FlatMapKeyFormatter, node *yaml.Node, results map[string]NodePair) {
	for i, childNode := range node.Content {
		// this is a value node
		if childNode.Kind == yaml.ScalarNode && (i%2 == 1) {
			keyNode := node.Content[i-1]
			key := keyNode.Value
			newPath := keyFormatter(path, key)

			results[newPath] = NodePair{
				Key:   node.Content[i-1],
				Value: childNode,
			}
		} else if childNode.Kind == yaml.MappingNode && (i%2 == 1) {
			key := node.Content[i-1].Value
			newPath := keyFormatter(path, key)

			flattenMap(newPath, keyFormatter, childNode, results)
			continue
		} else if childNode.Kind == yaml.MappingNode {
			flattenMap(path, keyFormatter, childNode, results)
			continue
		} else if childNode.Kind == yaml.SequenceNode {
			for i, sequenceNode := range childNode.Content {
				newPath := keyFormatter(path, fmt.Sprintf("[%v]", i))
				flattenMap(newPath, keyFormatter, sequenceNode, results)
			}
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
		return strconv.Atoi(value)
	}

	return value, nil
}

// GetYamlStringValue will attempt to get the key from the input mapping and return it as a lowercase string
// If the key does not exist, nil is returned, with no error
// If the value exists, but is not a string, an error is returned
func GetYamlStringValue(mapping map[string]interface{}, key string) (*string, error) {
	value, ok := mapping[key]
	if !ok {
		return nil, nil
	}

	valueString, okString := value.(string)
	if !okString {
		return nil, fmt.Errorf("value is not a string")
	}

	result := strings.ToLower(valueString)

	return &result, nil
}
