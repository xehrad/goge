package lib

import (
	"go/ast"
	"log"
	"strings"

	"golang.org/x/tools/go/packages"
)

func (g *meta) loadPackage() {
	cfg := &packages.Config{
		Mode: packages.NeedName |
			packages.NeedFiles |
			packages.NeedCompiledGoFiles |
			packages.NeedSyntax |
			packages.NeedTypes |
			packages.NeedTypesInfo,
		Dir: g.libPath, // load the package in libPath or ./lib
	}

	pkgs, err := packages.Load(cfg, "./...")
	if err != nil {
		log.Fatalf("packages.Load: %v", err)
	}
	if packages.PrintErrors(pkgs) > 0 {
		log.Fatal("packages.Load returned errors")
	}
	g.packages = pkgs
}

func (m *meta) analysis() {
	for _, pkg := range m.packages {
		for _, f := range pkg.Syntax {
			for _, decl := range f.Decls {
				fn, ok := decl.(*ast.FuncDecl)
				if !ok || fn.Doc == nil {
					continue
				}
				for _, c := range fn.Doc.List {
					if !strings.HasPrefix(c.Text, FLAG_COMMENT_API) {
						continue
					}
					meta := parseComment(c.Text)

					// Expect func(c *fiber.Ctx, params *Struct) (*Resp, error) or ([]byte, error)
					if fn.Type.Params == nil || fn.Type.Params.NumFields() != 2 || fn.Type.Results == nil || fn.Type.Results.NumFields() < 1 {
						log.Fatalf("bad signature for %s: want func(*fiber.Ctx, *Params) (*Resp, error) or ([]byte, error)", fn.Name.Name)
					}

					// params type name
					paramsStar, ok := fn.Type.Params.List[1].Type.(*ast.StarExpr)
					if !ok {
						log.Fatalf("%s second param must be *Struct", fn.Name.Name)
					}
					paramsIdent, ok := paramsStar.X.(*ast.Ident)
					if !ok {
						log.Fatalf("%s second param must be *Struct (ident)", fn.Name.Name)
					}

					// First result type name: allow pointer-to-struct or []byte
					respIsBytes := false
					respTypeName := ""
					switch rt := fn.Type.Results.List[0].Type.(type) {
					case *ast.StarExpr:
						resIdent, ok := rt.X.(*ast.Ident)
						if !ok {
							log.Fatalf("%s first result must be *Struct (ident)", fn.Name.Name)
						}
						respTypeName = resIdent.Name
					case *ast.ArrayType:
						// Allow []byte (or []uint8) as raw bytes
						if id, ok := rt.Elt.(*ast.Ident); ok && (id.Name == "byte" || id.Name == "uint8") {
							respIsBytes = true
							respTypeName = "[]byte"
						} else {
							log.Fatalf("%s first result must be *Struct or []byte", fn.Name.Name)
						}
					default:
						log.Fatalf("%s first result must be *Struct or []byte", fn.Name.Name)
					}

					m.apis = append(m.apis, APIMeta{
						FuncName:    fn.Name.Name,
						ParamsType:  paramsIdent.Name,
						RespType:    respTypeName,
						RespIsBytes: respIsBytes,
						Method:      meta["method"],
						Path:        meta["path"],
					})
				}
			}
		}
	}
}

// Store struct definitions from syntax
func (m *meta) createStructType() {
	m.structs = make(map[string]*ast.StructType)

	for _, pkg := range m.packages {
		for _, f := range pkg.Syntax {
			for _, decl := range f.Decls {
				gd, ok := decl.(*ast.GenDecl)
				if !ok {
					continue
				}

				for _, spec := range gd.Specs {
					ts, ok := spec.(*ast.TypeSpec)
					if !ok {
						continue
					}
					if st, ok := ts.Type.(*ast.StructType); ok {
						m.structs[ts.Name.Name] = st
					}
				}
			}
		}
	}
}

func parseComment(comment string) map[string]string {
	comment = strings.TrimPrefix(comment, FLAG_COMMENT_API)
	parts := strings.Fields(comment)
	out := make(map[string]string, 3)
	for _, p := range parts {
		if kv := strings.SplitN(p, "=", 2); len(kv) == 2 {
			out[kv[0]] = kv[1]
		} else {
			out[p] = p
		}
	}
	return out
}

// collectFields flattens fields of a struct, including embedded ones.
func (m *meta) collectFields(st *ast.StructType) []*ast.Field {
	var fields []*ast.Field
	if st == nil || st.Fields == nil {
		return fields
	}
	for _, f := range st.Fields.List {
		// If the field is embedded (anonymous)
		if len(f.Names) == 0 {
			// Check if it's a struct we know
			if ident, ok := f.Type.(*ast.Ident); ok {
				if embedded, exists := m.structs[ident.Name]; exists {
					// Recurse into embedded struct
					fields = append(fields, m.collectFields(embedded)...)
					continue
				}
			}
		}
		fields = append(fields, f)
	}
	return fields
}



// fiberQueryMethodForType returns the Fiber query accessor method name for a given type.
// Used by the template engine to generate code like: c.QueryInt("name")
func fiberQueryMethodForType(expr ast.Expr) string {
    switch t := expr.(type) {
    case *ast.Ident:
        switch t.Name {
        case "int", "int64", "int32":
            return "QueryInt"
        case "float32", "float64":
            return "QueryFloat"
        case "bool":
            return "QueryBool"
        default:
            return "Query"
        }
    case *ast.SelectorExpr:
        return "Query"
    default:
        return "Query"
    }
}
