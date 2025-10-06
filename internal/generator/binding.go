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

type FieldBind struct {
	Name         string
	Kind         string // header|query|url|cookie
	Key          string
	QueryFunc    string
	DefaultValue string
	HasDefault   bool
	KindHint     valKind
}

// ExtractBindingsRecursive handles embedded structs (BaseUser)
func ExtractBindingsRecursive(pkg *scanner.PackageAPIs, st *astStruct) []FieldBind {
	binds := ExtractBindings(pkg, st.Struct)
	for _, f := range st.Fields().List {
		// Embedded struct (no field name)
		if len(f.Names) == 0 {
			if ident, ok := f.Type.(*ast.Ident); ok {
				if embedded := parseStructAST(pkg.PkgDir, ident.Name); embedded != nil {
					binds = append(binds, ExtractBindingsRecursive(pkg, embedded)...)
				}
			}
		}
	}
	return binds
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

		if v, ok := stag.Lookup("gogeHeader"); ok {
			key, def := parseBindingKey(v)
			_, vk := fiberQueryMethodAndKind(f.Type)
			addBind("header", key, def, "", vk)
		}
		if v, ok := stag.Lookup("gogeQuery"); ok {
			key, def := parseBindingKey(v)
			method, vk := fiberQueryMethodAndKind(f.Type)
			addBind("query", key, def, method, vk)
		}
		if v, ok := stag.Lookup("gogeUrl"); ok {
			key, _ := parseBindingKey(v)
			_, vk := fiberQueryMethodAndKind(f.Type)
			addBind("url", key, "", "", vk)
		}
		if v, ok := stag.Lookup("gogeCookie"); ok {
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
