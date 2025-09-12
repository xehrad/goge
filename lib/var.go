package lib

import (
	"errors"
	"go/ast"

	"golang.org/x/tools/go/packages"
)

type (
	meta struct {
		apis     []APIMeta
		structs  map[string]*ast.StructType
		packages []*packages.Package

		libName string
		libPath string
	}

	APIMeta struct {
		FuncName    string
		ParamsType  string
		RespType    string
		RespIsBytes bool
		Method      string
		Path        string
	}
)

const (
    FILE_OUTPUT_NAME          = "api_generated.go"
    OPEN_API_FILE_OUTPUT_NAME = "openapi.json"
    OPEN_API_META_FILE_NAME   = "meta.json"
    FLAG_COMMENT_API          = "//goge:api "
    TAG_HEADER                = "gogeHeader"
    TAG_QUERY                 = "gogeQuery"
    TAG_URL                   = "gogeUrl"
)

var ErrNoTemplate = errors.New("template not found")
