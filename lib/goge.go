package lib

import (
	"encoding/json"
	"fmt"
	"go/ast"
	"go/format"
	"io"
	"log"
	"os"
	"path"
	"reflect"
	"regexp"
	"strings"

	"golang.org/x/tools/go/packages"
)

func Run() {
	goge := NewGoge()

	goge.loadPackage()
	goge.findAPI()
	goge.createStructType()

	src, err := goge.generate()
	if err != nil {
		log.Fatalf("format failed: %v", err)
	} else if err := os.WriteFile(FILE_OUTPUT_NAME, src, 0644); err != nil {
		log.Fatalf("could not write file: %v", err)
	}

	openapi, err := goge.generateOpenAPI()
	if err != nil {
		log.Fatalf("could not marshal OpenAPI spec: %v", err)
	} else if err := os.WriteFile("openapi.json", openapi, 0644); err != nil {
		log.Fatalf("could not write OpenAPI spec: %v", err)
	}
}

func NewGoge() *gogeMeta {
	g := new(gogeMeta)
	g.libName = "lib"
	g.libPath = "./lib"
	if len(os.Args) > 1 {
		g.libName = os.Args[1]
	}
	if len(os.Args) > 2 {
		g.libPath = os.Args[2]
	}
	return g
}

func (g *gogeMeta) loadPackage() {
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

// --------- ANALYSIS (using *packages.Package.Syntax) ----------

func (g *gogeMeta) findAPI() {
	for _, pkg := range g.packages {
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

					g.apis = append(g.apis, APIMeta{
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
func (g *gogeMeta) createStructType() {
	g.structs = make(map[string]*ast.StructType)

	for _, pkg := range g.packages {
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
						g.structs[ts.Name.Name] = st
					}
				}
			}
		}
	}
}

// --------- GENERATION (handlers) ----------

func (g *gogeMeta) generate() ([]byte, error) {
	var b strings.Builder
	fmt.Fprintf(&b, IMPORT_HEADER, g.libName)

	for _, api := range g.apis {
		method := strings.ToUpper(api.Method)
		if method == "POST" || method == "PUT" || method == "PATCH" {
			fmt.Fprintf(&b, HANDLER_BODY_FUNCTION_START, api.FuncName, api.ParamsType)
		} else {
			fmt.Fprintf(&b, HANDLER_FUNCTION_START, api.FuncName, api.ParamsType)
		}

		// Reflect on struct tags
		st := g.structs[api.ParamsType]
		if st != nil {
			for _, field := range st.Fields.List {
				if field.Tag == nil || len(field.Names) == 0 {
					continue
				}
				name := field.Names[0].Name
				tag := reflect.StructTag(strings.Trim(field.Tag.Value, "`"))

				if val, ok := tag.Lookup(TAG_HEADER); ok {
					fmt.Fprintf(&b, VAR_SET_HEADER, name, val)
				}
				if val, ok := tag.Lookup(TAG_QUERY); ok {
					fmt.Fprintf(&b, VAR_SET_QUERY, name, val)
				}
				if val, ok := tag.Lookup(TAG_URL); ok {
					fmt.Fprintf(&b, VAR_SET_URL, name, val)
				}
			}
		}

		fmt.Fprintf(&b, RETURN_JSON, api.FuncName)
		b.WriteString(HANDLER_FUNCTION_END)
	}

	g.addMainFunction(&b)
	return format.Source([]byte(b.String()))
}

func (g *gogeMeta) addMainFunction(b io.Writer) {
	var handler strings.Builder
	for _, api := range g.apis {
		method := strings.ToUpper(api.Method)
		fmt.Fprintf(&handler, FUNC_HANDLER_SET, method, api.Path, api.FuncName)
	}
	fmt.Fprintf(b, MAIN_FUNCTION_ROUTER, handler.String())
}

// --------- OpenAPI (unchanged logic, but used with new loader) ----------

func (g *gogeMeta) generateOpenAPI() ([]byte, error) {
	paths := make(map[string]map[string]any)

	for _, api := range g.apis {
		method := strings.ToLower(api.Method)

		parameters := []map[string]any{}
		if st := g.structs[api.ParamsType]; st != nil {
			for _, field := range st.Fields.List {
				if field.Tag == nil || len(field.Names) == 0 {
					continue
				}
				tag := reflect.StructTag(strings.Trim(field.Tag.Value, "`"))

				if val, ok := tag.Lookup(TAG_QUERY); ok {
					parameters = append(parameters, map[string]any{
						"name": val,
						"in":   "query",
						"schema": map[string]any{
							"type": "string",
						},
					})
				}
				if val, ok := tag.Lookup(TAG_HEADER); ok {
					parameters = append(parameters, map[string]any{
						"name": val,
						"in":   "header",
						"schema": map[string]any{
							"type": "string",
						},
					})
				}
				if val, ok := tag.Lookup(TAG_URL); ok {
					parameters = append(parameters, map[string]any{
						"name":     val,
						"in":       "path",
						"required": true,
						"schema": map[string]any{
							"type": "string",
						},
					})
				}
			}
		}

		oasPath := toOpenAPIPath(api.Path)
		if paths[oasPath] == nil {
			paths[oasPath] = make(map[string]any)
		}
		paths[oasPath][method] = map[string]any{
			"tags":       []string{getBaseRoot(oasPath)},
			"summary":    api.FuncName,
			"parameters": parameters,
			"responses": map[string]any{
				"200": map[string]any{
					"description": "Success",
				},
			},
		}
	}

	openapi := map[string]any{
		"openapi": "3.0.4",
		"info": map[string]any{
			"title":   "Goge API",
			"version": "1.0.0",
			"description": `
This is a sample Pet Store Server based on the OpenAPI 3.0 specification.  
You can find out more about Swagger at https://swagger.io. 
In the third iteration of the pet store, we've switched to the design first approach!
You can now help us improve the API whether it's by making changes to the definition itself or to the code.`,
		},
		"paths": paths,
	}

	return json.MarshalIndent(openapi, "", "  ")
}

// GetBaseRoot returns the first segment of a path after the initial slash.
func getBaseRoot(p string) string {
	cleanPath := path.Clean(p)
	trimmed := strings.Trim(cleanPath, "/")
	if trimmed == "" {
		return ""
	}
	return strings.Split(trimmed, "/")[0]
}

// Replace :param with {param}
func toOpenAPIPath(p string) string {
	re := regexp.MustCompile(`:([a-zA-Z0-9_]+)`)
	return re.ReplaceAllString(p, `{$1}`)
}
