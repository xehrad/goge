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
		"paths":   paths,
		"openapi": "3.0.1",
		"info": map[string]any{
			"title":          "Kloud.Team API Core",
			"description":    "The Kloud.Team API provides a comprehensive solution for managing applications within a Platform as a Service (PaaS) and Continuous Integration/Continuous Deployment (CI/CD) environment. This API allows users to create, update, retrieve, and manage application configurations, deployments, and related resources such as ConfigMaps and pipelines. The API supports fine-grained operations for specific applications, enabling users to start, stop, or modify applications and retrieve logs or other metadata in real-time. Additionally, the API facilitates managing user roles, project configurations, and namespaces, ensuring seamless integration across multiple development environments.",
			"termsOfService": "https://kloud.team",
			"contact": map[string]any{
				"name": "API Support",
				"url":  "https://kloud.team/support",
			},
			"version": "2.0.1",
		},
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
