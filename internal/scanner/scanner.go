package scanner

import (
	"maps"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

type Endpoint struct {
	PkgDir         string // filesystem directory of the package
	PkgName        string
	RecvName       string // service receiver name (e.g. "*service" or "service")
	MethodName     string // e.g. Register
	HTTPMethod     string // GET/POST/PUT/DELETE
	Path           string // e.g. /user/:id
	InputIsStruct  bool
	InputTypeExpr  string // as written in signature, e.g. "*domain.RegisterUser" or "string"
	ReturnTypeExpr string
	InputIsPtr     bool
	Imports        map[string]string // alias => import path (for generated file)
	ManualFunc     string
}

type PackageAPIs struct {
	PkgDir    string
	PkgName   string
	Imports   map[string]string
	Endpoints []Endpoint
}

// Parse `//goge:api method=POST path=/user`
var gogeRe = regexp.MustCompile(`^goge:api\s+method=([A-Z]+)\s+path=([^\s]+)(?:\s+manual_func=([A-Za-z0-9_]+))?\s*$`)

func Scan(root string) (map[string]*PackageAPIs, error) {
	result := map[string]*PackageAPIs{}
	fset := token.NewFileSet()

	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			// skip vendor, .git, node_modules, build dirs
			base := filepath.Base(path)
			if strings.HasPrefix(base, ".") || base == "vendor" || base == "node_modules" {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") {
			return nil
		}
		if strings.HasSuffix(path, "_gen.go") || strings.HasSuffix(path, "handler_gen.go") {
			return nil
		}

		fileAst, err := parser.ParseFile(fset, path, nil, parser.ParseComments)
		if err != nil {
			return fmt.Errorf("parse %s: %w", path, err)
		}

		pkgDir := filepath.Dir(path)
		pkgName := fileAst.Name.Name

		// collect imports (alias->path)
		imports := map[string]string{}
		for _, im := range fileAst.Imports {
			ip := strings.Trim(im.Path.Value, `"`)
			alias := ""
			if im.Name != nil {
				alias = im.Name.Name
			} else {
				// default alias is last path element
				parts := strings.Split(ip, "/")
				alias = parts[len(parts)-1]
			}
			imports[alias] = ip
		}

		// check funcs
		for _, decl := range fileAst.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok || fn.Doc == nil {
				continue
			}
			var httpMethod, httpPath, manualFunc string
			for _, c := range fn.Doc.List {
				txt := strings.TrimPrefix(strings.TrimSpace(c.Text), "//")
				if m := gogeRe.FindStringSubmatch(txt); m != nil {
					httpMethod, httpPath = m[1], m[2]
					manualFunc = m[3]
					break
				}
			}
			if httpMethod == "" {
				continue
			} // not annotated

			// only methods with receiver (service methods)
			if fn.Recv == nil || len(fn.Recv.List) == 0 {
				return fmt.Errorf("%s: //goge:api must be on a method with receiver", path)
			}
			recvExpr := exprString(fn.Recv.List[0].Type)

			// input param
			if fn.Type.Params == nil || len(fn.Type.Params.List) != 1 {
				return fmt.Errorf("%s: %s must have exactly ONE input param (DTO)", path, fn.Name.Name)
			}
			inTypeExpr := exprString(fn.Type.Params.List[0].Type)
			inputIsPtr := strings.HasPrefix(inTypeExpr, "*")
			inputBase := strings.TrimPrefix(inTypeExpr, "*")
			// heuristic: consider struct-like input if contains a dot (pkg.Type) OR first letter uppercase
			inputIsStruct := strings.Contains(inputBase, ".") || (len(inputBase) > 0 && strings.ToUpper(inputBase[:1]) == inputBase[:1])

			// extract return type (first one)
			retTypeExpr := "any"
			if fn.Type.Results != nil && len(fn.Type.Results.List) > 0 {
				retTypeExpr = exprString(fn.Type.Results.List[0].Type)
			}

			ep := Endpoint{
				PkgDir:         pkgDir,
				PkgName:        pkgName,
				RecvName:       recvExpr,
				MethodName:     fn.Name.Name,
				HTTPMethod:     httpMethod,
				Path:           httpPath,
				InputIsStruct:  inputIsStruct,
				InputTypeExpr:  inTypeExpr,
				InputIsPtr:     inputIsPtr,
				Imports:        imports,
				ReturnTypeExpr: retTypeExpr,
				ManualFunc:     manualFunc,
			}

			pkg := result[pkgDir]
			if pkg == nil {
				pkg = &PackageAPIs{PkgDir: pkgDir, PkgName: pkgName, Imports: map[string]string{}}
				result[pkgDir] = pkg
			}
			// merge imports
			maps.Copy(pkg.Imports, imports)
			pkg.Endpoints = append(pkg.Endpoints, ep)
		}
		return nil
	})

	if err != nil {
		return nil, err
	}
	return result, nil
}

func exprString(e ast.Expr) string {
	switch v := e.(type) {
	case *ast.Ident:
		return v.Name
	case *ast.StarExpr:
		return "*" + exprString(v.X)
	case *ast.SelectorExpr:
		return exprString(v.X) + "." + v.Sel.Name
	case *ast.ArrayType:
		return "[]" + exprString(v.Elt)
	case *ast.MapType:
		return "map[" + exprString(v.Key) + "]" + exprString(v.Value)
	case *ast.InterfaceType:
		return "interface{}"
	case *ast.FuncType:
		return "func"
	default:
		// fallback; good enough for our use
		return fmt.Sprintf("%T", e)
	}
}
