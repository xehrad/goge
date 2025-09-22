package lib

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/format"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"text/template"
)

func isBodyParse(api *metaAPI) bool {
	return strings.EqualFold(api.Method, "POST") || strings.EqualFold(api.Method, "PUT") || strings.EqualFold(api.Method, "PATCH")
}

// generateWithTemplates renders the entire output file using text/template.
// It prefers user-provided templates via env var GOGE_TEMPLATES, falling back
// to embedded defaults for Fiber.
func (m *meta) generateWithTemplates() ([]byte, error) {
	// Build bindings and flags (BodyParse) for each API
	for i := range m.APIs {
		api := &m.APIs[i]
		if api.ParamsExpr == "" {
			api.ParamsExpr = api.ParamsType
		}
		api.BodyParse = isBodyParse(api)
		if st := m.structs[api.ParamsType]; st != nil {
			for _, field := range m.collectFields(st) {
				if field.Tag == nil || len(field.Names) == 0 {
					continue
				}
				name := field.Names[0].Name
				if api.ParamsPkg != "" && !ast.IsExported(name) {
					continue
				}
				stag := reflect.StructTag(strings.Trim(field.Tag.Value, "`"))
				if val, ok := stag.Lookup(_TAG_HEADER); ok {
					key, opts := parseTagBindingValue(val)
					bind := fieldBind{
						Name:      name,
						Kind:      "header",
						Key:       key,
						ValueKind: "string",
					}
					if raw, ok := opts["default"]; ok {
						raw = strings.TrimSpace(raw)
						if goLit, valid := defaultLiteralForKind("string", raw); valid {
							bind.HasDefault = true
							bind.DefaultRaw = raw
							bind.DefaultGoValue = goLit
						}
					}
					api.Binds = append(api.Binds, bind)
				}
				if val, ok := stag.Lookup(_TAG_QUERY); ok {
					key, opts := parseTagBindingValue(val)
					valueKind := inferValueKind(field.Type)
					bind := fieldBind{
						Name:      name,
						Kind:      "query",
						Key:       key,
						QueryFunc: fiberQueryMethodForType(field.Type),
						ValueKind: valueKind,
					}
					if raw, ok := opts["default"]; ok {
						raw = strings.TrimSpace(raw)
						if goLit, valid := defaultLiteralForKind(valueKind, raw); valid {
							bind.HasDefault = true
							bind.DefaultRaw = raw
							bind.DefaultGoValue = goLit
						}
					}
					api.Binds = append(api.Binds, bind)
				}
				if val, ok := stag.Lookup(_TAG_URL); ok {
					key, _ := parseTagBindingValue(val)
					bind := fieldBind{
						Name:      name,
						Kind:      "url",
						Key:       key,
						ValueKind: "string",
					}
					api.Binds = append(api.Binds, bind)
				}
			}
		}
	}

	m.buildExtraImports()

	// Generate OpenAPI JSON now so we can embed it into the code
	b, _ := m.generateOpenAPIWithTemplates()

	ss := strings.ReplaceAll(fmt.Sprint(b), " ", ",")
	m.OpenAPIFileByte = ss[1 : len(ss)-1]
	m.SwaggerHTML = SWAGGER_HTML

	// Load template
	tplPath := os.Getenv("GOGE_TEMPLATES")
	var content string
	var err error
	if tplPath != "" {
		content, err = loadExternalTemplate(filepath.Join(tplPath, "fiber", "api_file.go.tmpl"))
		if err != nil {
			return nil, fmt.Errorf("load external template: %w", err)
		}
	} else {
		content, err = loadEmbeddedTemplate("fiber/api_file.go.tmpl")
		if err != nil {
			return nil, fmt.Errorf("load embedded template: %w", err)
		}
	}

	tpl, err := template.New("api_file").Parse(content)
	if err != nil {
		return nil, err
	}

	var buf bytes.Buffer
	if err := tpl.Execute(&buf, m); err != nil {
		return nil, err
	}
	return format.Source(buf.Bytes())
}

// loadExternalTemplate reads a template file from disk.
func loadExternalTemplate(path string) (string, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

// loadEmbeddedTemplate returns the embedded default content.
func loadEmbeddedTemplate(name string) (string, error) {
	b, err := templatesFS.ReadFile(filepath.ToSlash(filepath.Join("templates", name)))
	if err != nil {
		return "", err
	}
	return string(b), nil
}

func (m *meta) buildExtraImports() {
	if len(m.imports) == 0 {
		m.ExtraImports = nil
		return
	}
	keys := make([]string, 0, len(m.imports))
	for alias := range m.imports {
		keys = append(keys, alias)
	}
	sort.Strings(keys)
	imports := make([]importRef, 0, len(keys))
	for _, alias := range keys {
		imports = append(imports, importRef{Alias: alias, Path: m.imports[alias]})
	}
	m.ExtraImports = imports
}
