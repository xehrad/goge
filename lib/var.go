package lib

import (
	"go/ast"

	"golang.org/x/tools/go/packages"
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

type (
	meta struct {
		APIs         []metaAPI
		structs      map[string]*ast.StructType
		packages     []*packages.Package
		imports      map[string]string
		loadedPkgs   map[string]bool
		ExtraImports []importRef

		LibName string
		libPath string

		// Generated artifacts for templates
		OpenAPIFileByte string
		SwaggerHTML     string
	}

	metaAPI struct {
		FuncName    string
		ParamsType  string
		ParamsExpr  string
		ParamsPkg   string
		RespType    string
		RespPackage string
		RespIsBytes bool
		Method      string
		Path        string
		BodyParse   bool
		// Bindings collected from struct tags
		Binds []fieldBind
	}

	importRef struct {
		Alias string
		Path  string
	}

	fieldBind struct {
		Name string
		Kind string // header|query|url
		Key  string
		// For Fiber at the moment: Query, QueryInt, QueryFloat, QueryBool
		QueryFunc string
		// Default handling for bindings (currently used for query params)
		HasDefault     bool
		DefaultRaw     string
		DefaultGoValue string
		ValueKind      string // string|int|float|bool (used for OpenAPI defaults)
	}
)

type (
	openAPIData struct {
		ComponentsSchemasJSON string // JSON string for the value of components.schemas
		OpenAPIMetaJSON       string // JSON string for the value of Info and other metadata
		Paths                 []openAPIPath
		Tags                  []openAPITag `json:"tags,omitempty"`
	}

	openAPITag struct {
		Name        string `json:"name,omitempty"`
		Description string `json:"description,omitempty"`
	}

	openAPIMeta struct {
		Openapi      string               `json:"openapi,omitempty"`
		Info         *openAPIInfo         `json:"info,omitempty"`
		ExternalDocs *openAPIExternalDocs `json:"externalDocs,omitempty"`
		Servers      []struct {
			URL string `json:"url,omitempty"`
		} `json:"servers,omitempty"`
	}
	openAPIExternalDocs struct {
		Description string `json:"description,omitempty"`
		URL         string `json:"url,omitempty"`
	}
	openAPIInfo struct {
		Title          string          `json:"title,omitempty"`
		Description    string          `json:"description,omitempty"`
		TermsOfService string          `json:"termsOfService,omitempty"`
		Contact        *openAPIContact `json:"contact,omitempty"`
		License        *openAPILicense `json:"license,omitempty"`
		Version        string          `json:"version,omitempty"`
	}

	openAPILicense struct {
		Name string `json:"name,omitempty"`
		URL  string `json:"url,omitempty"`
	}
	openAPIContact struct {
		Name  string `json:"name,omitempty"`
		URL   string `json:"url,omitempty"`
		Email string `json:"email,omitempty"`
	}

	openAPIPath struct {
		Path    string
		Tag     string
		Methods []openAPIMethod
	}

	openAPIMethod struct {
		Method     string // lower-case: get, post, ...
		Summary    string
		Parameters []openAPIParam
		// Optional references
		RequestBodyRef   string // components schema name
		ResponseRef      string // components schema name
		ResponseIsBinary bool   // if true, use application/octet-stream string/binary
	}

	openAPIParam struct {
		Name        string
		In          string // query, header, path
		Required    bool
		Type        string // string (default)
		HasDefault  bool
		DefaultJSON string
	}
)

const SWAGGER_HTML = `
<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="utf-8" />
  <meta name="viewport" content="width=device-width, initial-scale=1" />
  <meta name="description" content="SwaggerUI" />
  <title>SwaggerUI</title>
  <link rel="stylesheet" href="https://unpkg.com/swagger-ui-dist@5.29.0/swagger-ui.css" />
  <style>html,body,#swagger-ui{height:100%;margin:0}</style>
</head>
<body>
  <div id="swagger-ui"></div>
  <script src="https://unpkg.com/swagger-ui-dist@5.29.0/swagger-ui-bundle.js" crossorigin></script>
  <script>
    window.onload = () => {
      window.ui = SwaggerUIBundle({
        url: 'swagger.json',
        dom_id: '#swagger-ui'
      });
    };
  </script>
</body>
</html>`
