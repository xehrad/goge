package lib

import (
	"go/ast"
	"strings"
	"testing"
)

func TestGeneratorInMemory(t *testing.T) {
	g := &gogeMeta{
		apis: []APIMeta{
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
					{Names: []*ast.Ident{{Name: "ID"}}, Tag: &ast.BasicLit{Value: "`" + TAG_URL + ":\"id\"`"}},
					{Names: []*ast.Ident{{Name: "Auth"}}, Tag: &ast.BasicLit{Value: "`" + TAG_HEADER + ":\"Authorization\"`"}},
					{Names: []*ast.Ident{{Name: "Filter"}}, Tag: &ast.BasicLit{Value: "`" + TAG_QUERY + ":\"filter\"`"}},
				}},
			},
		},
		libName: "lib",
	}

	src, err := g.generate()
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

	openapi, err := g.generateOpenAPI()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(openapi), `"in": "path"`) {
		t.Errorf("expected path param in OpenAPI, got:\n%s", string(openapi))
	}
}
