package lib

import (
	"bytes"
	"encoding/json"
	"go/ast"
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
	// JSON string for the value of components.schemas
	ComponentsSchemasJSON string
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
	// Optional references
	RequestBodyRef   string // components schema name
	ResponseRef      string // components schema name
	ResponseIsBinary bool   // if true, use application/octet-stream string/binary
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
					params = append(
						params,
						OpenAPIParam{
							Name: val,
							In:   "query",
							Type: "string",
						},
					)
				}
				if val, ok := tag.Lookup(TAG_HEADER); ok {
					params = append(
						params,
						OpenAPIParam{
							Name: val,
							In:   "header",
							Type: "string",
						},
					)
				}
				if val, ok := tag.Lookup(TAG_URL); ok {
					params = append(
						params,
						OpenAPIParam{
							Name:     val,
							In:       "path",
							Required: true,
							Type:     "string",
						},
					)
				}
			}
		}

		me := OpenAPIMethod{Method: method, Summary: api.FuncName, Parameters: params}
		// Request body for methods that parse body
		if isBodyParse(&api) {
			if _, ok := m.structs[api.ParamsType]; ok {
				me.RequestBodyRef = api.ParamsType
			}
		}
		// Response schema
		if api.RespIsBytes {
			me.ResponseIsBinary = true
		} else if _, ok := m.structs[api.RespType]; ok && api.RespType != "" {
			me.ResponseRef = api.RespType
		}
		byPath[oasPath].Methods = append(byPath[oasPath].Methods, me)
	}

	// Flatten to slice to have deterministic template iteration order
	paths := make([]OpenAPIPath, 0, len(byPath))
	for _, p := range byPath {
		paths = append(paths, *p)
	}

	// Build components.schemas for used types (params + responses)
	used := map[string]bool{}
	for _, p := range paths {
		for _, me := range p.Methods {
			if me.RequestBodyRef != "" {
				used[me.RequestBodyRef] = true
			}
			if me.ResponseRef != "" {
				used[me.ResponseRef] = true
			}
		}
	}
	schemas := m.buildSchemas(used)
	schemasJSONBytes, _ := json.MarshalIndent(schemas, "", "  ")

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
		Paths:                 paths,
		ComponentsSchemasJSON: string(schemasJSONBytes),
	}
}

// buildSchemas builds OpenAPI component schemas for the provided type names (and their dependencies).
func (m *meta) buildSchemas(used map[string]bool) map[string]any {
	out := map[string]any{}
	visited := map[string]bool{}
	var add func(name string)
	add = func(name string) {
		if visited[name] {
			return
		}
		visited[name] = true
		st := m.structs[name]
		if st == nil {
			return
		}
		props := map[string]any{}
		for _, f := range m.collectFields(st) {
			if len(f.Names) == 0 {
				// anonymous embedded fields are flattened by collectFields
				continue
			}
			// Ignore fields that are bound to header/query/url parameters
			if f.Tag != nil {
				stag := reflect.StructTag(strings.Trim(f.Tag.Value, "`"))
				if _, ok := stag.Lookup(TAG_HEADER); ok {
					continue
				}
				if _, ok := stag.Lookup(TAG_QUERY); ok {
					continue
				}
				if _, ok := stag.Lookup(TAG_URL); ok {
					continue
				}
			}
			fName := f.Names[0].Name
			schema, refName := m.schemaForExpr(f.Type)
			if refName != "" {
				add(refName)
			}
			props[fName] = schema
		}
		out[name] = map[string]any{
			"type":       "object",
			"properties": props,
		}
	}
	for name := range used {
		add(name)
	}
	return out
}

// schemaForExpr converts a Go AST expression to a (schema, refName) pair.
// If refName != "", schema is a $ref to that component.
func (m *meta) schemaForExpr(expr ast.Expr) (map[string]any, string) {
	switch t := expr.(type) {
	case *ast.Ident:
		switch t.Name {
		case "string":
			return map[string]any{"type": "string"}, ""
		case "bool":
			return map[string]any{"type": "boolean"}, ""
		case "int", "int32":
			return map[string]any{"type": "integer", "format": "int32"}, ""
		case "int64":
			return map[string]any{"type": "integer", "format": "int64"}, ""
		case "float32":
			return map[string]any{"type": "number", "format": "float"}, ""
		case "float64":
			return map[string]any{"type": "number", "format": "double"}, ""
		default:
			// If it's a known struct, reference it; otherwise fallback to string
			if _, ok := m.structs[t.Name]; ok {
				return map[string]any{"$ref": "#/components/schemas/" + t.Name}, t.Name
			}
			return map[string]any{"type": "string"}, ""
		}
	case *ast.StarExpr:
		if id, ok := t.X.(*ast.Ident); ok {
			if _, ok := m.structs[id.Name]; ok {
				return map[string]any{"$ref": "#/components/schemas/" + id.Name}, id.Name
			}
			// pointer to primitive
			return m.schemaForExpr(id)
		}
		return map[string]any{"type": "string"}, ""
	case *ast.ArrayType:
		itemSchema, ref := m.schemaForExpr(t.Elt)
		arr := map[string]any{"type": "array", "items": itemSchema}
		return arr, ref
	case *ast.MapType:
		// additionalProperties
		valSchema, ref := m.schemaForExpr(t.Value)
		obj := map[string]any{"type": "object", "additionalProperties": valSchema}
		return obj, ref
	case *ast.SelectorExpr:
		// external types → string
		return map[string]any{"type": "string"}, ""
	default:
		return map[string]any{"type": "string"}, ""
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
