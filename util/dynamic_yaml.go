package util

import (
	"github.com/iancoleman/strcase"
	"gopkg.in/yaml.v2"
	"io/ioutil"
	"log"
	"os"
	"reflect"
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
	data map[interface{}]interface{}
}

func LoadDynamicYaml(filePath string) (*DynamicYaml, error) {
	rawFileData, err := ioutil.ReadFile(filePath)
	if err != nil {
		return nil, err
	}

	data := make(map[interface{}]interface{})
	if err := yaml.Unmarshal(rawFileData, &data); err != nil {
		return nil, err
	}

	dynamicYaml := &DynamicYaml{
		data:data,
	}

	return dynamicYaml, nil
}

// Gets a value by using dot notation.
// cats.persian.quantity
func (d *DynamicYaml) Get(path string) interface{} {
	return d.GetByString(path, ".")
}

func (d *DynamicYaml) GetData() map[interface{}]interface{} {
	return d.data
}

func (d *DynamicYaml) GetByString(path, separator string) interface{} {
	keys := strings.Split(path, separator)

	return d.GetByParts(keys...)
}

func (d *DynamicYaml) GetByParts(keys ...string) interface{} {
	return getValue(d.data, keys, -1)
}

func (d *DynamicYaml) PutByParts(value interface{}, keys ...string) {
	queryObj := d.data

	for i, key := range keys {
		if i == len(keys) - 1 {
			queryObj[key] = value
			continue
		}

		if _, ok := queryObj[key]; !ok {
			queryObj[key] = make(map[interface{}]interface{})
		}

		queryObj = queryObj[key].(map[interface{}]interface{})
	}
}

func (d *DynamicYaml) PutByString(value interface{}, path, separator string) {
	parts := strings.Split(path, separator)
	d.PutByParts(value, parts...)
}

// Put a value by using dot notation.
// cats.persian.quantity
func (d *DynamicYaml) Put(value interface{}, path string) {
	d.PutByString(value, path, ".")
}

func (d *DynamicYaml) Flatten(keyFormatter FlatMapKeyFormatter) map[string]interface{} {
	results := make(map[string]interface{})

	flattenMap("", keyFormatter, d.data, results)

	return results
}

func flattenMap(path string, keyFormatter FlatMapKeyFormatter,  obj map[interface{}]interface{}, results map[string]interface{}) {
	for key := range obj {
		newKeyAsString, stringOk := key.(string)
		if !stringOk {
			log.Printf("[error] key '%v' is not a string", key)
			continue
		}

		newPath := keyFormatter(path, newKeyAsString)
		newObj := obj[key]

		objAsMap, ok := newObj.(map[interface{}]interface{})
		if !ok {
			results[newPath] = newObj
			continue
		}

		flattenMap(newPath, keyFormatter, objAsMap, results)
	}
}

// Adapted from https://stackoverflow.com/a/47198590
func getValue(obj map[interface{}]interface{}, keys []string, indexOfElementInArray int) interface{} {
	//fmt.Printf("--- Root object:\n%v\n\n", obj)
	queryObj := obj
	for i := range keys {
		if queryObj == nil {
			break
		}
		if i == len(keys)-1 {
			break
		}
		key := keys[i]
		//fmt.Printf("--- querying for sub object keyed by %v\n", key)
		if queryObj[key] != nil {
			queryObj = queryObj[key].(map[interface{}]interface{})
			//fmt.Printf("--- Sub object keyed by %v :\n%v\n\n", key, queryObj)
		} else {
			//fmt.Printf("--- No sub object keyed by %v :\n%v\n\n", key)
			break
		}
	}
	if queryObj != nil {
		lastKey := keys[len(keys)-1]
		//fmt.Printf("--- querying for value keyed by %v\n", lastKey)

		if queryObj[lastKey] != nil {
			objType := reflect.TypeOf(queryObj[lastKey])
			//fmt.Printf("Type of value %v\n", objType)
			if objType.String() == "[]interface {}" {
				//fmt.Printf("Object is a array %v\n", objType)
				tempArr := queryObj[lastKey].([]interface{})
				//fmt.Printf("Length of array is %v\n", len(tempArr))
				if indexOfElementInArray >= 0 && indexOfElementInArray < len(tempArr) {
					return queryObj[lastKey].([]interface{})[indexOfElementInArray]
				}
			} else {
				return queryObj[lastKey]
			}
		}
	}

	return nil
}

func (d *DynamicYaml) WriteToFile(file *os.File) error {
	data, err := yaml.Marshal(d.data)
	if err != nil {
		return err
	}

	_, err = file.Write(data)

	return err
}