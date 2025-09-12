package lib

import (
	"go/ast"

	"golang.org/x/tools/go/packages"
)

type (
	meta struct {
		APIs     []metaAPI
		structs  map[string]*ast.StructType
		packages []*packages.Package

		LibName string
		libPath string
	}
)

const (
	_FILE_OUTPUT_NAME          = "api_generated.go"
	_OPEN_API_FILE_OUTPUT_NAME = "openapi.json"
	_OPEN_API_META_FILE_NAME   = "meta.json"
	_FLAG_COMMENT_API          = "//goge:api "
	_TAG_HEADER                = "gogeHeader"
	_TAG_QUERY                 = "gogeQuery"
	_TAG_URL                   = "gogeUrl"
)
