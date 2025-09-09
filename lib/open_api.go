package lib

import (
	"encoding/json"
	"path"
	"reflect"
	"regexp"
	"strings"
)

func (m *meta) generateOpenAPI() ([]byte, error) {
	paths := make(map[string]map[string]any)

	for _, api := range m.apis {
		method := strings.ToLower(api.Method)

		parameters := []map[string]any{}
		if st := m.structs[api.ParamsType]; st != nil {
			for _, field := range m.collectFields(st) {
				if field.Tag == nil || len(field.Names) == 0 {
					continue
				}
				tag := reflect.StructTag(strings.Trim(field.Tag.Value, "`"))

				if val, ok := tag.Lookup(TAG_QUERY); ok {
					parameters = append(parameters, map[string]any{
						"name": val,
						"in":   "query",
						"schema": map[string]any{
							"type": "string",
						},
					})
				}
				if val, ok := tag.Lookup(TAG_HEADER); ok {
					parameters = append(parameters, map[string]any{
						"name": val,
						"in":   "header",
						"schema": map[string]any{
							"type": "string",
						},
					})
				}
				if val, ok := tag.Lookup(TAG_URL); ok {
					parameters = append(parameters, map[string]any{
						"name":     val,
						"in":       "path",
						"required": true,
						"schema": map[string]any{
							"type": "string",
						},
					})
				}
			}
		}

		oasPath := toOpenAPIPath(api.Path)
		if paths[oasPath] == nil {
			paths[oasPath] = make(map[string]any)
		}
		paths[oasPath][method] = map[string]any{
			"tags":       []string{getBaseRoot(oasPath)},
			"summary":    api.FuncName,
			"parameters": parameters,
			"responses": map[string]any{
				"200": map[string]any{
					"description": "Success",
				},
			},
		}
	}

	openapi := map[string]any{
		"openapi": "3.0.4",
		"info": map[string]any{
			"title":   "Goge API",
			"version": "1.0.0",
			"description": `
This is a sample Pet Store Server based on the OpenAPI 3.0 specification.  
You can find out more about Swagger at https://swagger.io. 
In the third iteration of the pet store, we've switched to the design first approach!
You can now help us improve the API whether it's by making changes to the definition itself or to the code.`,
		},
		"paths": paths,
	}

	return json.MarshalIndent(openapi, "", "  ")
}

// GetBaseRoot returns the first segment of a path after the initial slash.
func getBaseRoot(p string) string {
	cleanPath := path.Clean(p)
	trimmed := strings.Trim(cleanPath, "/")
	if trimmed == "" {
		return ""
	}
	return strings.Split(trimmed, "/")[0]
}

// Replace :param with {param}
func toOpenAPIPath(p string) string {
	re := regexp.MustCompile(`:([a-zA-Z0-9_]+)`)
	return re.ReplaceAllString(p, `{$1}`)
}
