package lib

import (
	"go/ast"
	"strings"
	"testing"
)

func TestGeneratorInMemory(t *testing.T) {
	g := &meta{
		APIs: []metaAPI{
			{
				FuncName:   "GetUser",
				ParamsType: "UserParams",
				RespType:   "UserResp",
				Method:     "GET",
				Path:       "/users/:id",
			},
		},
		structs: map[string]*ast.StructType{
			"UserParams": {
				Fields: &ast.FieldList{List: []*ast.Field{
					{Names: []*ast.Ident{{Name: "ID"}}, Tag: &ast.BasicLit{Value: "`" + _TAG_URL + ":\"id\"`"}},
					{Names: []*ast.Ident{{Name: "Auth"}}, Tag: &ast.BasicLit{Value: "`" + _TAG_HEADER + ":\"Authorization\"`"}},
					{Names: []*ast.Ident{{Name: "Filter"}}, Tag: &ast.BasicLit{Value: "`" + _TAG_QUERY + ":\"filter\"`"}},
				}},
			},
		},
		LibName: "lib",
	}

	src, err := g.generateWithTemplates()
	if err != nil {
		t.Fatal(err)
	}
	code := string(src)
	if !strings.Contains(code, `c.Params("id")`) {
		t.Errorf("expected URL param, got:\n%s", code)
	}
	if !strings.Contains(code, `c.Get("Authorization")`) {
		t.Errorf("expected header param, got:\n%s", code)
	}
	if !strings.Contains(code, `c.Query("filter")`) {
		t.Errorf("expected query param, got:\n%s", code)
	}

	openapi, err := g.generateOpenAPIWithTemplates()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(openapi), `"in": "path"`) {
		t.Errorf("expected path param in OpenAPI, got:\n%s", string(openapi))
	}
}

func TestGenerateWithExternalImport(t *testing.T) {
	g := &meta{
		APIs: []metaAPI{
			{
				FuncName:    "Ping",
				ParamsType:  "PingParams",
				ParamsExpr:  "foo.PingParams",
				ParamsPkg:   "foo",
				RespIsBytes: true,
				Method:      "GET",
				Path:        "/ping",
			},
		},
		imports: map[string]string{
			"foo": "example.com/foo",
		},
		LibName: "lib",
	}

	src, err := g.generateWithTemplates()
	if err != nil {
		t.Fatal(err)
	}
	code := string(src)
	if !strings.Contains(code, `foo "example.com/foo"`) {
		t.Errorf("expected external import, got:\n%s", code)
	}
	if !strings.Contains(code, "new(foo.PingParams)") {
		t.Errorf("expected qualified params instantiation, got:\n%s", code)
	}
}

func TestGenerateWithExternalImportBindings(t *testing.T) {
	g := &meta{
		APIs: []metaAPI{
			{
				FuncName:   "NamespaceGet",
				ParamsType: "ReqDatacenter",
				ParamsExpr: "request.ReqDatacenter",
				ParamsPkg:  "request",
				RespType:   "BaseResponse",
				Method:     "GET",
				Path:       "/namespace/:namespace",
			},
		},
		structs: map[string]*ast.StructType{
			"ReqDatacenter": {
				Fields: &ast.FieldList{List: []*ast.Field{
					{Names: []*ast.Ident{{Name: "Namespace"}}, Tag: &ast.BasicLit{Value: "`" + _TAG_URL + ":\"namespace\"`"}},
					{Names: []*ast.Ident{{Name: "DatacenterID"}}, Tag: &ast.BasicLit{Value: "`" + _TAG_HEADER + ":\"KT_DATACENTER_ID,default=default\"`"}},
				}},
			},
		},
		imports: map[string]string{
			"request": "example.com/request",
		},
		LibName: "lib",
	}

	src, err := g.generateWithTemplates()
	if err != nil {
		t.Fatal(err)
	}
	code := string(src)
	if !strings.Contains(code, `req.Namespace = c.Params("namespace")`) {
		t.Errorf("expected URL binding, got:\n%s", code)
	}
	if !strings.Contains(code, `req.DatacenterID = c.Get("KT_DATACENTER_ID"`) {
		t.Errorf("expected header binding, got:\n%s", code)
	}
}
