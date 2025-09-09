package lib

import (
	"fmt"
	"go/format"
	"io"
	"log"
	"os"
	"path"
	"reflect"
	"strings"
)

func Run() {
	goge := NewGoge()
	goge.loadPackage()
	goge.analysis()
	goge.createStructType()

	srcPath := path.Join(goge.libPath, FILE_OUTPUT_NAME)
	src, err := goge.generate()
	if err != nil {
		log.Fatalf("format failed: %v", err)
	} else if err := os.WriteFile(srcPath, src, 0644); err != nil {
		log.Fatalf("could not write file: %v", err)
	}

	openapiPath := path.Join(goge.libPath, OPEN_API_FILE_OUTPUT_NAME)
	openapi, err := goge.generateOpenAPI()
	if err != nil {
		log.Fatalf("could not marshal OpenAPI spec: %v", err)
	} else if err := os.WriteFile(openapiPath, openapi, 0644); err != nil {
		log.Fatalf("could not write OpenAPI spec: %v", err)
	}
}

func NewGoge() *meta {
	g := new(meta)
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

// --------- GENERATION (handlers) ----------

func (m *meta) generate() ([]byte, error) {
	var b strings.Builder
	fmt.Fprintf(&b, IMPORT_HEADER, m.libName)

	for _, api := range m.apis {
		method := strings.ToUpper(api.Method)
		if method == "POST" || method == "PUT" || method == "PATCH" {
			fmt.Fprintf(&b, HANDLER_BODY_FUNCTION_START, api.FuncName, api.ParamsType)
		} else {
			fmt.Fprintf(&b, HANDLER_FUNCTION_START, api.FuncName, api.ParamsType)
		}

		// Reflect on struct tags
		st := m.structs[api.ParamsType]
		if st != nil {
			for _, field := range m.collectFields(st) {
				if field.Tag == nil || len(field.Names) == 0 {
					continue
				}
				name := field.Names[0].Name
				tag := reflect.StructTag(strings.Trim(field.Tag.Value, "`"))

				if val, ok := tag.Lookup(TAG_HEADER); ok {
					fmt.Fprintf(&b, VAR_SET_HEADER, name, val)
				}
				if val, ok := tag.Lookup(TAG_QUERY); ok {
					qFunc := fiberQueryFuncForType(field.Type)
					fmt.Fprintf(&b, qFunc, name, val)
				}
				if val, ok := tag.Lookup(TAG_URL); ok {
					fmt.Fprintf(&b, VAR_SET_URL, name, val)
				}
			}
		}

		fmt.Fprintf(&b, RETURN_JSON, api.FuncName)
		b.WriteString(HANDLER_FUNCTION_END)
	}

	m.addMainFunction(&b)
	return format.Source([]byte(b.String()))
}

func (g *meta) addMainFunction(b io.Writer) {
	var handler strings.Builder
	for _, api := range g.apis {
		method := strings.ToUpper(api.Method)
		fmt.Fprintf(&handler, FUNC_HANDLER_SET, method, api.Path, api.FuncName)
	}
	fmt.Fprintf(b, MAIN_FUNCTION_ROUTER, handler.String())
}
