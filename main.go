package main

// import "github.com/xehrad/goge/lib"

// func main() {
// 	lib.Run()
// }

import (
	"flag"
	"log"

	"github.com/xehrad/goge/internal/generator"
	"github.com/xehrad/goge/internal/scanner"
)

func main() {

	root := flag.String("root", ".", "project root to scan")
	flag.Parse()

	apis, err := scanner.Scan(*root)
	if err != nil {
		log.Fatalf("scan error: %v", err)
	}
	if len(apis) == 0 {
		log.Println("no //goge:api annotations found. nothing to do.")
		return
	}
	if err := generator.Generate(*root, apis); err != nil {
		log.Fatalf("generate error: %v", err)
	}
	log.Printf("goge: generated handlers for %d packages\n", len(apis))
}
