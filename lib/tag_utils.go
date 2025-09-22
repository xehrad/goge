package lib

import (
	"go/ast"
	"strconv"
	"strings"
)

// parseTagBindingValue splits a tag value like "name,default=12" into the key
// ("name") and additional options keyed by name (e.g. map["default"] = "12").
func parseTagBindingValue(val string) (key string, options map[string]string) {
	parts := strings.Split(val, ",")
	key = strings.TrimSpace(parts[0])
	options = make(map[string]string, len(parts)-1)
	for _, part := range parts[1:] {
		segment := strings.TrimSpace(part)
		if segment == "" {
			continue
		}
		kv := strings.SplitN(segment, "=", 2)
		if len(kv) != 2 {
			continue
		}
		options[strings.TrimSpace(kv[0])] = strings.TrimSpace(kv[1])
	}
	return key, options
}

// inferValueKind maps a struct field type to a simplified kind used for
// choosing Fiber query helpers and formatting defaults. Unknown types default
// to "string" since Fiber works with strings by default.
func inferValueKind(expr ast.Expr) string {
	switch t := expr.(type) {
	case *ast.Ident:
		switch t.Name {
		case "int", "int64", "int32", "int16", "int8":
			return "int"
		case "float64", "float32":
			return "float"
		case "bool":
			return "bool"
		case "string":
			return "string"
		default:
			return "string"
		}
	default:
		return "string"
	}
}

// defaultLiteralForKind validates and returns a Go literal for the provided raw
// default value according to the simplified kind.
func defaultLiteralForKind(kind, raw string) (string, bool) {
	switch kind {
	case "string":
		return strconv.Quote(raw), true
	case "bool":
		if _, err := strconv.ParseBool(raw); err != nil {
			return "", false
		}
		return raw, true
	case "int":
		if _, err := strconv.ParseInt(raw, 10, 64); err != nil {
			return "", false
		}
		return raw, true
	case "float":
		if _, err := strconv.ParseFloat(raw, 64); err != nil {
			return "", false
		}
		return raw, true
	default:
		return "", false
	}
}

// defaultJSONLiteral returns the value serialized appropriately for embedding
// in JSON templates (e.g. "\"value\"" or "12").
func defaultJSONLiteral(kind, raw string) (string, bool) {
	switch kind {
	case "string":
		return strconv.Quote(raw), true
	case "bool":
		if _, err := strconv.ParseBool(raw); err != nil {
			return "", false
		}
		return raw, true
	case "int":
		if _, err := strconv.ParseInt(raw, 10, 64); err != nil {
			return "", false
		}
		return raw, true
	case "float":
		if _, err := strconv.ParseFloat(raw, 64); err != nil {
			return "", false
		}
		return raw, true
	default:
		return "", false
	}
}
