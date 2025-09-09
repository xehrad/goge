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

					// Expect func(c *fiber.Ctx, params *Struct) (*Resp, error)
					if fn.Type.Params == nil || fn.Type.Params.NumFields() != 2 || fn.Type.Results == nil || fn.Type.Results.NumFields() < 1 {
						log.Fatalf("bad signature for %s: want func(*fiber.Ctx, *Params) (*Resp, error)", fn.Name.Name)
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

					// first result type name (*Resp)
					resStar, ok := fn.Type.Results.List[0].Type.(*ast.StarExpr)
					if !ok {
						log.Fatalf("%s first result must be *Resp", fn.Name.Name)
					}
					resIdent, ok := resStar.X.(*ast.Ident)
					if !ok {
						log.Fatalf("%s first result must be *Resp (ident)", fn.Name.Name)
					}

					m.apis = append(m.apis, APIMeta{
						FuncName:   fn.Name.Name,
						ParamsType: paramsIdent.Name,
						RespType:   resIdent.Name,
						Method:     meta["method"],
						Path:       meta["path"],
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
