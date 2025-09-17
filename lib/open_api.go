package lib

import (
	"bytes"
	"encoding/json"
	"go/ast"
	"log"
	"os"
	"path"
	"path/filepath"
	"reflect"
	"regexp"
	"slices"
	"strings"
	"text/template"
	"unicode"
)

func (m *meta) generateOpenAPIWithTemplates() (_ []byte, err error) {
	var (
		tplPath = os.Getenv("GOGE_TEMPLATES")
		data    = m.buildOpenAPIData()
		content string
		buf     bytes.Buffer
	)

	// Resolve template source (external path or embedded)
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

	tpl, err := template.New("openapi").Parse(content)
	if err != nil {
		return nil, err
	}
	if err := tpl.Execute(&buf, data); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func (m *meta) buildOpenAPIData() *openAPIData {
	output := new(openAPIData)

	// Group methods by path
	byPath := map[string]*openAPIPath{}
	for _, api := range m.APIs {
		method := strings.ToLower(api.Method)
		oasPath := toOpenAPIPath(api.Path)
		if byPath[oasPath] == nil {
			byPath[oasPath] = &openAPIPath{Path: oasPath, Tag: getBaseRoot(oasPath)}
		}

		// Parameters
		params := []openAPIParam{}
		if st := m.structs[api.ParamsType]; st != nil {
			for _, field := range m.collectFields(st) {
				if field.Tag == nil || len(field.Names) == 0 {
					continue
				}
				tag := reflect.StructTag(strings.Trim(field.Tag.Value, "`"))
				if val, ok := tag.Lookup(_TAG_QUERY); ok {
					params = append(
						params,
						openAPIParam{
							Name: val,
							In:   "query",
							Type: "string",
						},
					)
				}
				if val, ok := tag.Lookup(_TAG_HEADER); ok {
					params = append(
						params,
						openAPIParam{
							Name: val,
							In:   "header",
							Type: "string",
						},
					)
				}
				if val, ok := tag.Lookup(_TAG_URL); ok {
					params = append(
						params,
						openAPIParam{
							Name:     val,
							In:       "path",
							Required: true,
							Type:     "string",
						},
					)
				}
			}
		}

		me := openAPIMethod{Method: method, Summary: api.FuncName, Parameters: params}
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
	output.Paths = make([]openAPIPath, 0, len(byPath))

	tagsMap := make(map[string]any, 64)
	for _, p := range byPath {
		output.Paths = append(output.Paths, *p)
		tagsMap[p.Tag] = nil
	}

	// Sort Tags
	output.Tags = sortedTagsKeys(tagsMap)

	// Build components.schemas for used types (params + responses)
	used := map[string]bool{}
	for _, p := range output.Paths {
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
	schemasJSONBytes, _ := json.Marshal(schemas)
	output.ComponentsSchemasJSON = string(schemasJSONBytes)

	// Load meta from file if present
	oMeta := m.loadOpenAPIMeta()
	oMetaJson, _ := json.Marshal(oMeta)
	oMetaJson = oMetaJson[1 : len(oMetaJson)-1] // remove '{' and '}' of JSON
	output.OpenAPIMetaJSON = string(oMetaJson)

	return output
}

// loadOpenAPIMeta reads meta.json from the lib path to fill OpenAPI version and Info.
func (m *meta) loadOpenAPIMeta() *openAPIMeta {
	// Defaults
	oMeta := new(openAPIMeta)
	oMeta.Info = new(openAPIInfo)
	oMeta.Openapi = "3.0.4"
	oMeta.Info.Title = "API Core"
	oMeta.Info.Description = "Goge has generated this API. To modify this description, please add the file meta.json."
	oMeta.Info.TermsOfService = "https://github.com/xehrad/goge"
	oMeta.Info.Version = "0.0.1"

	b, err := os.ReadFile(_OPEN_API_META_FILE_NAME)
	if err != nil {
		log.Printf("Warning, open file %s", _OPEN_API_META_FILE_NAME)
		return oMeta
	}

	if err := json.Unmarshal(b, oMeta); err != nil {
		log.Printf("Warning unmarshal json file %s, err:%s", _OPEN_API_META_FILE_NAME, err.Error())
		return oMeta
	}

	return oMeta
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
				if _, ok := stag.Lookup(_TAG_HEADER); ok {
					continue
				}
				if _, ok := stag.Lookup(_TAG_QUERY); ok {
					continue
				}
				if _, ok := stag.Lookup(_TAG_URL); ok {
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

// GetBaseRoot returns the first segment of a path after the initial slash.
func getBaseRoot(p string) string {
	cleanPath := path.Clean(p)
	trimmed := strings.Trim(cleanPath, "/")
	if trimmed == "" {
		return ""
	}

	tag := strings.Split(trimmed, "/")[0]
	return ToCamelCaseTitle(tag)
}

// Replace :param with {param}
func toOpenAPIPath(p string) string {
	re := regexp.MustCompile(`:([a-zA-Z0-9_]+)`)
	return re.ReplaceAllString(p, `{$1}`)
}

func sortedTagsKeys(m map[string]any) []openAPITag {
	if len(m) == 0 {
		return nil
	}

	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	slices.Sort(keys)

	out := make([]openAPITag, 0, len(keys))
	for _, t := range keys {
		out = append(out, openAPITag{Name: t})
	}

	return out
}

func ToCamelCaseTitle(s string) string {
	// Split on underscores or dashes
	parts := strings.FieldsFunc(s, func(r rune) bool {
		return r == '_' || r == '-'
	})

	for i, p := range parts {
		if len(p) == 0 {
			continue
		}
		runes := []rune(p)
		// Capitalize first letter
		runes[0] = unicode.ToUpper(runes[0])
		// Lowercase the rest
		for j := 1; j < len(runes); j++ {
			runes[j] = unicode.ToLower(runes[j])
		}
		parts[i] = string(runes)
	}

	return strings.Join(parts, " ")
}
