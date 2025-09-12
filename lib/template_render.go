package lib

import (
	"bytes"
	"fmt"
	"go/format"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"text/template"
)

// templateData represents the data model passed to renderer templates.
type templateData struct {
	Package string
	APIs    []metaAPI
}

type metaAPI struct {
	FuncName    string
	ParamsType  string
	RespType    string
	RespIsBytes bool
	Method      string
	Path        string
	BodyParse   bool
	// Bindings collected from struct tags
	Binds []fieldBind
}

type fieldBind struct {
	Name string
	Kind string // header|query|url
	Key  string
	// For Fiber at the moment: Query, QueryInt, QueryFloat, QueryBool
	QueryFunc string
}

func isBodyParse(api *APIMeta) bool {
	return strings.EqualFold(api.Method, "POST") || strings.EqualFold(api.Method, "PUT") || strings.EqualFold(api.Method, "PATCH")
}

// generateWithTemplates renders the entire output file using text/template.
// It prefers user-provided templates via env var GOGE_TEMPLATES, falling back
// to embedded defaults for Fiber.
func (m *meta) generateWithTemplates() ([]byte, error) {
	data := templateData{Package: m.libName}

	for _, api := range m.apis {
		a := metaAPI{
			FuncName:    api.FuncName,
			ParamsType:  api.ParamsType,
			RespType:    api.RespType,
			RespIsBytes: api.RespIsBytes,
			Method:      strings.ToUpper(api.Method),
			Path:        api.Path,
			BodyParse:   isBodyParse(&api),
		}

		if st := m.structs[api.ParamsType]; st != nil {
			for _, field := range m.collectFields(st) {
				if field.Tag == nil || len(field.Names) == 0 {
					continue
				}
				name := field.Names[0].Name
				stag := reflect.StructTag(strings.Trim(field.Tag.Value, "`"))
				if val, ok := stag.Lookup(TAG_HEADER); ok {
					a.Binds = append(
						a.Binds,
						fieldBind{
							Name: name,
							Kind: "header",
							Key:  val,
						},
					)
				}
				if val, ok := stag.Lookup(TAG_QUERY); ok {
					a.Binds = append(
						a.Binds,
						fieldBind{
							Name:      name,
							Kind:      "query",
							Key:       val,
							QueryFunc: fiberQueryMethodForType(field.Type),
						},
					)
				}
				if val, ok := stag.Lookup(TAG_URL); ok {
					a.Binds = append(
						a.Binds,
						fieldBind{
							Name: name,
							Kind: "url",
							Key:  val,
						},
					)
				}
			}
		}
		data.APIs = append(data.APIs, a)
	}

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
	if err := tpl.Execute(&buf, data); err != nil {
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
