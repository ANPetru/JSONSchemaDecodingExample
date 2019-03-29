package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"strconv"
	"strings"

	"github.com/jaracil/ei"
	"github.com/qri-io/jsonschema"
)

type Property struct {
	Name, JSONType                         string
	Required                               bool
	MaxLength, MinLength, Maximum, Minimum int
}

func initBoolean(name string, required bool) Property {
	return Property{Name: name, JSONType: "boolean", Required: required, MaxLength: -1,
		MinLength: -1, Maximum: -1, Minimum: -1}
}

func initNumber(name string, required bool, maximum, minimum int) Property {
	return Property{Name: name, JSONType: "number", Required: required, MaxLength: -1,
		MinLength: -1, Maximum: maximum, Minimum: minimum}
}

func initString(name string, required bool, maxLength, minLength int) Property {
	return Property{Name: name, JSONType: "string", Required: required, MaxLength: maxLength,
		MinLength: minLength, Maximum: -1, Minimum: -1}
}

var rs *jsonschema.RootSchema
var requiredKeys []string
var properties []Property

func main() {
	getProperiesFromSchema("schema.json")
}

func getProperiesFromSchema(schema string) {
	schemaData, _ := ioutil.ReadFile(schema)

	rs = &jsonschema.RootSchema{}
	if err := json.Unmarshal(schemaData, rs); err != nil {
		panic("unmarshal schema: " + err.Error())
	}

	prop := rs.JSONProp("properties").(*jsonschema.Properties)
	keys := prop.JSONChildren()
	procesObject("root", keys)
	fmt.Println(properties)
}

func getRequiredKeys(path string) []string {
	paths := strings.Split(path, "/")
	jsonPath := rs.JSONProp("properties")
	for _, p := range paths {
		if p != "root" {
			jsonPath = jsonPath.(*jsonschema.Properties).JSONProp(p)

		}
	}
	if req := jsonPath.(jsonschema.JSONPather).JSONProp("required"); req != nil {

		reqs := getStringsFromRequired(req.(*jsonschema.Required))
		for i, r := range reqs {
			reqs[i] = path + "/" + r
		}
		return reqs
	}

	return nil
}

func getStringsFromRequired(req *jsonschema.Required) []string {
	str := fmt.Sprint(req)
	if str != "" && len(str) > 3 {
		return strings.Split(str[2:len(str)-1], " ")
	}
	return nil
}

func getDefinitions(defKey string) (prop *jsonschema.Properties, req *jsonschema.Required) {
	defs := rs.JSONProp("definitions").(jsonschema.Definitions).JSONChildren()
	for k, v := range defs {
		if k == defKey {
			if prop := v.JSONProp("properties"); prop != nil {
				if req := v.JSONProp("required"); req != nil {
					return prop.(*jsonschema.Properties), req.(*jsonschema.Required)
				}
				return prop.(*jsonschema.Properties), nil
			}
		}
	}
	return nil, nil
}

func procesObject(name string, keys map[string]jsonschema.JSONPather) {
	for k, v := range keys {
		if s := v.JSONProp("type"); s != nil {
			str := s.(*jsonschema.Type).String()
			switch str {
			case "object":
				req := getRequiredKeys(name + "/" + k)

				for _, v := range req {
					requiredKeys = append(requiredKeys, v)
				}
				procesObject(name+"/"+k, v.JSONProp("properties").(*jsonschema.Properties).JSONChildren())
			case "number":
				max := -1
				min := -1
				if maxJSON := v.JSONProp("maximum"); maxJSON != nil {
					pointer := maxJSON.(*jsonschema.Maximum)
					max, _ = strconv.Atoi(fmt.Sprintf("%.0f", *pointer))

				}
				if minJSON := v.JSONProp("minimum"); minJSON != nil {
					pointer := minJSON.(*jsonschema.Minimum)
					min, _ = strconv.Atoi(fmt.Sprintf("%.0f", *pointer))
				}
				saveNumber(name+"/"+k, max, min)
			case "string":
				maxLength := -1
				minLength := -1
				if mxlJ := v.JSONProp("maxLength"); mxlJ != nil {
					pointer := mxlJ.(*jsonschema.MaxLength)
					maxLength, _ = strconv.Atoi(fmt.Sprintf("%d", *pointer))
				}
				if mnLJ := v.JSONProp("minLength"); mnLJ != nil {
					pointer := mnLJ.(*jsonschema.MinLength)
					minLength, _ = strconv.Atoi(fmt.Sprintf("%d", *pointer))
				}
				saveString(name+"/"+k, maxLength, minLength)
			case "boolean":
				saveBoolean(name + "/" + k)
			}
		} else if s := v.JSONProp("allOf"); s != nil {
			processAllOf(name+"/"+k, s.(*jsonschema.AllOf).JSONChildren())
		} else if s := v.JSONProp("$ref"); s != "" {
			str := ei.N(s).StringZ()
			processReference(name+"/"+k, str[1:len(str)])
		}

	}
}

func processReference(name, refName string) {
	prop, req := getDefinitions(refName)
	reqs := getStringsFromRequired(req)
	for _, v := range reqs {
		requiredKeys = append(requiredKeys, name+"/"+v)
	}
	procesObject(name, prop.JSONChildren())
}

func processAllOf(name string, keys map[string]jsonschema.JSONPather) {
	for _, v := range keys {
		if s := v.JSONProp("properties"); s != nil {
			prop := s.(*jsonschema.Properties)
			procesObject(name, prop.JSONChildren())
		}
		if s := v.JSONProp("$ref"); s != "" {
			str := ei.N(s).StringZ()
			processReference(name, str[1:len(str)])
		}
	}
}

func saveBoolean(name string) {
	prop := initBoolean(name, getIsRequired(name))
	properties = append(properties, prop)
}

func saveString(name string, maxLength, minLength int) {
	prop := initString(name, getIsRequired(name), maxLength, minLength)
	properties = append(properties, prop)
}

func saveNumber(name string, max, min int) {
	prop := initNumber(name, getIsRequired(name), max, min)
	properties = append(properties, prop)
}

func getIsRequired(name string) bool {
	for _, v := range requiredKeys {
		if name == v {
			return true
		}
	}
	return false
}
