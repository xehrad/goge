package lib

import (
	"bytes"
	"os"
	"path"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"
	"text/template"
)

type OpenAPIData struct {
	OpenAPI string
	Info    OpenAPIInfo
	Paths   []OpenAPIPath
}

type OpenAPIInfo struct {
	Title          string
	Description    string
	TermsOfService string
	ContactName    string
	ContactURL     string
	Version        string
}

type OpenAPIPath struct {
	Path    string
	Tag     string
	Methods []OpenAPIMethod
}

type OpenAPIMethod struct {
	Method     string // lower-case: get, post, ...
	Summary    string
	Parameters []OpenAPIParam
}

type OpenAPIParam struct {
	Name     string
	In       string // query, header, path
	Required bool
	Type     string // string (default)
}

func (m *meta) buildOpenAPIData() OpenAPIData {
	// Group methods by path
	byPath := map[string]*OpenAPIPath{}

	for _, api := range m.apis {
		method := strings.ToLower(api.Method)
		oasPath := toOpenAPIPath(api.Path)
		if byPath[oasPath] == nil {
			byPath[oasPath] = &OpenAPIPath{Path: oasPath, Tag: getBaseRoot(oasPath)}
		}

		// Parameters
		params := []OpenAPIParam{}
		if st := m.structs[api.ParamsType]; st != nil {
			for _, field := range m.collectFields(st) {
				if field.Tag == nil || len(field.Names) == 0 {
					continue
				}
				tag := reflect.StructTag(strings.Trim(field.Tag.Value, "`"))
				if val, ok := tag.Lookup(TAG_QUERY); ok {
					params = append(params, OpenAPIParam{Name: val, In: "query", Type: "string"})
				}
				if val, ok := tag.Lookup(TAG_HEADER); ok {
					params = append(params, OpenAPIParam{Name: val, In: "header", Type: "string"})
				}
				if val, ok := tag.Lookup(TAG_URL); ok {
					params = append(params, OpenAPIParam{Name: val, In: "path", Required: true, Type: "string"})
				}
			}
		}

		me := OpenAPIMethod{Method: method, Summary: api.FuncName, Parameters: params}
		byPath[oasPath].Methods = append(byPath[oasPath].Methods, me)
	}

	// Flatten to slice to have deterministic template iteration order
	paths := make([]OpenAPIPath, 0, len(byPath))
	for _, p := range byPath {
		paths = append(paths, *p)
	}

	return OpenAPIData{
		OpenAPI: "3.0.1",
		Info: OpenAPIInfo{
			Title:          "Kloud.Team API Core",
			Description:    "The Kloud.Team API provides a comprehensive solution for managing applications within a Platform as a Service (PaaS) and Continuous Integration/Continuous Deployment (CI/CD) environment. This API allows users to create, update, retrieve, and manage application configurations, deployments, and related resources such as ConfigMaps and pipelines. The API supports fine-grained operations for specific applications, enabling users to start, stop, or modify applications and retrieve logs or other metadata in real-time. Additionally, the API facilitates managing user roles, project configurations, and namespaces, ensuring seamless integration across multiple development environments.",
			TermsOfService: "https://kloud.team",
			ContactName:    "API Support",
			ContactURL:     "https://kloud.team/support",
			Version:        "2.0.1",
		},
		Paths: paths,
	}
}

func (m *meta) generateOpenAPIWithTemplates() ([]byte, error) {
	data := m.buildOpenAPIData()

	// Resolve template source (external path or embedded)
	tplPath := os.Getenv("GOGE_TEMPLATES")
	var content string
	var err error
	if tplPath != "" {
		content, err = loadExternalTemplate(filepath.Join(tplPath, "fiber", "openapi.json.tmpl"))
		if err != nil {
			return nil, err
		}
	} else {
		content, err = loadEmbeddedTemplate("fiber/openapi.json.tmpl")
		if err != nil {
			return nil, err
		}
	}

	funcMap := template.FuncMap{
		"lower": strings.ToLower,
	}
	tpl, err := template.New("openapi").Funcs(funcMap).Parse(content)
	if err != nil {
		return nil, err
	}
	var buf bytes.Buffer
	if err := tpl.Execute(&buf, data); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
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
