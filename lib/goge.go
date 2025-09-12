package lib

import (
	"log"
	"os"
	"path"
)

func Run() {
	goge := NewGoge()
	goge.loadPackage()
	goge.analysis()
	goge.createStructType()

	srcPath := path.Join(goge.libPath, FILE_OUTPUT_NAME)
	// Prefer template-based generation when available
	src, err := goge.generateWithTemplates()
	if err != nil {
		log.Fatalf("format failed: %v", err)
	} else if err := os.WriteFile(srcPath, src, 0644); err != nil {
		log.Fatalf("could not write file: %v", err)
	}

	openapiPath := path.Join(goge.libPath, OPEN_API_FILE_OUTPUT_NAME)
	openapi, err := goge.generateOpenAPIWithTemplates()
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
