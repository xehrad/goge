package generator

import (
	"fmt"
	"go/ast"
	"reflect"
	"strings"

	"github.com/xehrad/goge/internal/scanner"
)

type valKind int

const (
	kindString valKind = iota
	kindInt
	kindFloat
	kindBool
)

const (
	_TAG_HEADER = "gogeHeader"
	_TAG_QUERY  = "gogeQuery"
	_TAG_URL    = "gogeUrl"
	_TAG_COOKIE = "gogeCookie"
)

type FieldBind struct {
	Name         string
	Kind         string // header|query|url|cookie
	Key          string
	QueryFunc    string
	DefaultValue string
	HasDefault   bool
	KindHint     valKind
}

// ExtractBindingsRecursive handles embedded structs
func ExtractBindingsRecursive(pkg *scanner.PackageAPIs, st *astStruct) []FieldBind {
	return extractBindingsRecursive(pkg, st, map[string]bool{})
}

func extractBindingsRecursive(pkg *scanner.PackageAPIs, st *astStruct, visited map[string]bool) []FieldBind {
	if st == nil || st.Struct == nil {
		return nil
	}

	if key := st.key(); key != "" {
		if visited[key] {
			return nil
		}
		visited[key] = true
	}

	binds := ExtractBindings(pkg, st.Struct)
	for _, f := range st.Fields() {
		if len(f.Names) != 0 {
			continue
		}
		embedded := resolveEmbeddedStruct(pkg, st, f.Type)
		if embedded == nil {
			continue
		}
		binds = append(binds, extractBindingsRecursive(pkg, embedded, visited)...)
	}
	return binds
}

func resolveEmbeddedStruct(pkg *scanner.PackageAPIs, owner *astStruct, expr ast.Expr) *astStruct {
	switch t := expr.(type) {
	case *ast.Ident:
		return owner.load(t.Name)
	case *ast.StarExpr:
		return resolveEmbeddedStruct(pkg, owner, t.X)
	case *ast.SelectorExpr:
		if ident, ok := t.X.(*ast.Ident); ok {
			if owner != nil && owner.ImportPath != "" {
				// For structs loaded from an import, reuse the same loader.
				return owner.load(t.Sel.Name)
			}
			if importPath, ok := pkg.Imports[ident.Name]; ok {
				moduleDir := findModuleRoot(pkg.PkgDir)
				if moduleDir == "" {
					moduleDir = pkg.PkgDir
				}
				return loadStructFromImport(importPath, t.Sel.Name, moduleDir)
			}
		}
	}
	return nil
}

func ExtractBindings(pkg *scanner.PackageAPIs, st *ast.StructType) []FieldBind {
	binds := []FieldBind{}
	if st == nil || st.Fields == nil {
		return binds
	}

	for _, f := range st.Fields.List {
		if len(f.Names) == 0 || f.Tag == nil {
			continue
		}
		name := f.Names[0].Name
		stag := reflect.StructTag(strings.Trim(f.Tag.Value, "`"))

		addBind := func(kind, key, def, qfunc string, vk valKind) {
			binds = append(binds, FieldBind{
				Name:         name,
				Kind:         kind,
				Key:          key,
				QueryFunc:    qfunc,
				DefaultValue: def,
				HasDefault:   def != "",
				KindHint:     vk,
			})
		}

		if v, ok := stag.Lookup(_TAG_HEADER); ok {
			key, def := parseBindingKey(v)
			_, vk := fiberQueryMethodAndKind(f.Type)
			addBind("header", key, def, "", vk)
		}
		if v, ok := stag.Lookup(_TAG_QUERY); ok {
			key, def := parseBindingKey(v)
			method, vk := fiberQueryMethodAndKind(f.Type)
			addBind("query", key, def, method, vk)
		}
		if v, ok := stag.Lookup(_TAG_URL); ok {
			key, _ := parseBindingKey(v)
			_, vk := fiberQueryMethodAndKind(f.Type)
			addBind("url", key, "", "", vk)
		}
		if v, ok := stag.Lookup(_TAG_COOKIE); ok {
			key, def := parseBindingKey(v)
			_, vk := fiberQueryMethodAndKind(f.Type)
			addBind("cookie", key, def, "", vk)
		}
	}
	return binds
}

func parseBindingKey(v string) (key string, def string) {
	parts := strings.Split(v, ",")
	key = strings.TrimSpace(parts[0])
	if len(parts) > 1 {
		for _, p := range parts[1:] {
			pair := strings.SplitN(p, "=", 2)
			if len(pair) == 2 && strings.TrimSpace(pair[0]) == "default" {
				def = strings.TrimSpace(pair[1])
			}
		}
	}
	return
}

func fiberQueryMethodAndKind(expr ast.Expr) (method string, vk valKind) {
	switch t := expr.(type) {
	case *ast.Ident:
		switch t.Name {
		case "int", "int8", "int16", "int32", "int64",
			"uint", "uint8", "uint16", "uint32", "uint64":
			return "QueryInt", kindInt
		case "float32", "float64":
			return "QueryFloat", kindFloat
		case "bool":
			return "QueryBool", kindBool
		default:
			return "Query", kindString
		}
	default:
		return "Query", kindString
	}
}

func defaultLiteral(v string, k valKind) string {
	switch k {
	case kindInt:
		return v
	case kindFloat:
		return v
	case kindBool:
		// normalize lower
		vl := strings.ToLower(v)
		if vl != "true" && vl != "false" {
			vl = "false"
		}
		return vl
	default:
		return fmt.Sprintf("%q", v)
	}
}

func BuildBindCode(binds []FieldBind) string {
	var sb strings.Builder
	for _, b := range binds {
		switch b.Kind {
		case "header":
			if b.HasDefault {
				fmt.Fprintf(&sb, "\treq.%s = c.Get(%q, %s)\n",
					b.Name, b.Key, defaultLiteral(b.DefaultValue, kindString))
			} else {
				fmt.Fprintf(&sb, "\treq.%s = c.Get(%q)\n", b.Name, b.Key)
			}
		case "query":
			if b.HasDefault {
				fmt.Fprintf(&sb, "\treq.%s = c.%s(%q, %s)\n",
					b.Name, b.QueryFunc, b.Key, defaultLiteral(b.DefaultValue, b.KindHint))
			} else {
				fmt.Fprintf(&sb, "\treq.%s = c.%s(%q)\n", b.Name, b.QueryFunc, b.Key)
			}
		case "url":
			fmt.Fprintf(&sb, "\treq.%s = c.Params(%q)\n", b.Name, b.Key)
		case "cookie":
			if b.HasDefault {
				fmt.Fprintf(&sb, "\treq.%s = c.Cookies(%q, %s)\n",
					b.Name, b.Key, defaultLiteral(b.DefaultValue, kindString))
			} else {
				fmt.Fprintf(&sb, "\treq.%s = c.Cookies(%q)\n", b.Name, b.Key)
			}
		}
	}
	return sb.String()
}
