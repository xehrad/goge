package lib

import (
	"fmt"
	"go/ast"
	"log"
	"path"
	"strconv"
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
	g.loadedPkgs = make(map[string]bool, len(pkgs))
	for _, pkg := range pkgs {
		if pkg != nil && pkg.PkgPath != "" {
			g.loadedPkgs[pkg.PkgPath] = true
		}
	}
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
					if !strings.HasPrefix(c.Text, _FLAG_COMMENT_API) {
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
					paramsTypeName, paramsExpr, paramsPkg := m.resolveStructType(fn.Name.Name, f, "second param", paramsStar.X, true)

					// First result type name: allow pointer-to-struct or []byte
					respIsBytes := false
					respTypeName := ""
					respPkg := ""
					switch rt := fn.Type.Results.List[0].Type.(type) {
					case *ast.StarExpr:
						respTypeName, _, respPkg = m.resolveStructType(fn.Name.Name, f, "first result", rt.X, false)
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

					m.APIs = append(m.APIs, metaAPI{
						FuncName:    fn.Name.Name,
						ParamsType:  paramsTypeName,
						ParamsExpr:  paramsExpr,
						ParamsPkg:   paramsPkg,
						RespType:    respTypeName,
						RespPackage: respPkg,
						RespIsBytes: respIsBytes,
						Method:      meta["method"],
						Path:        meta["path"],
					})
				}
			}
		}
	}
}

func (m *meta) resolveStructType(fnName string, file *ast.File, context string, expr ast.Expr, registerImport bool) (string, string, string) {
	switch t := expr.(type) {
	case *ast.Ident:
		return t.Name, t.Name, ""
	case *ast.SelectorExpr:
		pkgIdent, ok := t.X.(*ast.Ident)
		if !ok || pkgIdent.Name == "" {
			log.Fatalf("%s %s must reference an imported type (package identifier)", fnName, context)
		}
		alias := pkgIdent.Name
		pathValue, ok := m.importPathForAlias(file, alias)
		if !ok {
			log.Fatalf("%s %s references unknown import alias %q", fnName, context, alias)
		}
		if registerImport {
			m.addImport(alias, pathValue)
			m.loadAdditionalPackage(pathValue)
		}
		return t.Sel.Name, fmt.Sprintf("%s.%s", alias, t.Sel.Name), alias
	default:
		log.Fatalf("%s %s must be a struct identifier", fnName, context)
	}
	return "", "", ""
}

func (m *meta) loadAdditionalPackage(pathValue string) {
	if pathValue == "" {
		return
	}
	if m.loadedPkgs == nil {
		m.loadedPkgs = make(map[string]bool)
	}
	if m.loadedPkgs[pathValue] {
		return
	}
	cfg := &packages.Config{
		Mode: packages.NeedName |
			packages.NeedFiles |
			packages.NeedCompiledGoFiles |
			packages.NeedSyntax |
			packages.NeedTypes |
			packages.NeedTypesInfo,
	}
	pkgs, err := packages.Load(cfg, pathValue)
	if err != nil {
		log.Printf("goge warning: load package %s failed: %v", pathValue, err)
		m.loadedPkgs[pathValue] = true
		return
	}
	if packages.PrintErrors(pkgs) > 0 {
		log.Printf("goge warning: package %s reported errors", pathValue)
	}
	m.packages = append(m.packages, pkgs...)
	for _, pkg := range pkgs {
		if pkg != nil && pkg.PkgPath != "" {
			m.loadedPkgs[pkg.PkgPath] = true
		}
	}
}

func (m *meta) importPathForAlias(file *ast.File, alias string) (string, bool) {
	for _, spec := range file.Imports {
		pathValue, err := strconv.Unquote(spec.Path.Value)
		if err != nil {
			log.Fatalf("invalid import path %s: %v", spec.Path.Value, err)
		}
		name := path.Base(pathValue)
		if spec.Name != nil {
			name = spec.Name.Name
		}
		if name == alias {
			return pathValue, true
		}
	}
	return "", false
}

func (m *meta) addImport(alias, importPath string) {
	if alias == "" {
		return
	}
	if m.imports == nil {
		m.imports = make(map[string]string)
	}
	if existing, ok := m.imports[alias]; ok {
		if existing != importPath {
			log.Fatalf("conflicting import aliases %q for %s and %s", alias, existing, importPath)
		}
		return
	}
	m.imports[alias] = importPath
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
	comment = strings.TrimPrefix(comment, _FLAG_COMMENT_API)
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
			// Check if it's a struct we know (including selector and pointer types)
			if embedded, ok := m.lookupStructForEmbeddedField(f.Type); ok && embedded != nil {
				// Prevent infinite recursion on self-embedding types
				if embedded == st {
					continue
				}
				fields = append(fields, m.collectFields(embedded)...)
				continue
			}
		}
		fields = append(fields, f)
	}
	return fields
}

// lookupStructForEmbeddedField resolves the struct definition for an embedded field.
func (m *meta) lookupStructForEmbeddedField(expr ast.Expr) (*ast.StructType, bool) {
	switch t := expr.(type) {
	case *ast.Ident:
		st, ok := m.structs[t.Name]
		return st, ok
	case *ast.SelectorExpr:
		st, ok := m.structs[t.Sel.Name]
		return st, ok
	case *ast.StarExpr:
		return m.lookupStructForEmbeddedField(t.X)
	default:
		return nil, false
	}
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
