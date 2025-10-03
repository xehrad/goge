package lib

import (
	"go/ast"
	"reflect"
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
					{Names: []*ast.Ident{{Name: "internalSecret"}}, Tag: &ast.BasicLit{Value: "`" + _TAG_HEADER + ":\"Internal\"`"}},
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
	if strings.Contains(code, "internalSecret") {
		t.Errorf("unexpected binding for unexported field, got:\n%s", code)
	}
}

func TestGenerateQueryDefaultsAndKinds(t *testing.T) {
	g := &meta{
		APIs: []metaAPI{{
			FuncName:   "Search",
			ParamsType: "SearchParams",
			Method:     "GET",
			Path:       "/search",
		}},
		structs: map[string]*ast.StructType{
			"SearchParams": {
				Fields: &ast.FieldList{List: []*ast.Field{
					{Names: []*ast.Ident{{Name: "Count"}}, Type: &ast.Ident{Name: "int"}, Tag: &ast.BasicLit{Value: "`" + _TAG_QUERY + ":\"count,default=42\"`"}},
					{Names: []*ast.Ident{{Name: "Precision"}}, Type: &ast.Ident{Name: "float64"}, Tag: &ast.BasicLit{Value: "`" + _TAG_QUERY + ":\"precision,default=0.75\"`"}},
					{Names: []*ast.Ident{{Name: "Active"}}, Type: &ast.Ident{Name: "bool"}, Tag: &ast.BasicLit{Value: "`" + _TAG_QUERY + ":\"active,default=true\"`"}},
					{Names: []*ast.Ident{{Name: "Name"}}, Type: &ast.Ident{Name: "string"}, Tag: &ast.BasicLit{Value: "`" + _TAG_QUERY + ":\"name,default=guest\"`"}},
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
	if !strings.Contains(code, `c.QueryInt("count", 42)`) {
		t.Errorf("expected QueryInt with default, got:\n%s", code)
	}
	if !strings.Contains(code, `c.QueryFloat("precision", 0.75)`) {
		t.Errorf("expected QueryFloat with default, got:\n%s", code)
	}
	if !strings.Contains(code, `c.QueryBool("active", true)`) {
		t.Errorf("expected QueryBool with default, got:\n%s", code)
	}
	if !strings.Contains(code, `c.Query("name", "guest")`) {
		t.Errorf("expected Query with string default, got:\n%s", code)
	}

	openapi, err := g.generateOpenAPIWithTemplates()
	if err != nil {
		t.Fatal(err)
	}
	opi := string(openapi)
	if !strings.Contains(opi, `"name": "count"`) || !strings.Contains(opi, `"default": 42`) {
		t.Errorf("expected OpenAPI default for count, got:\n%s", opi)
	}
	if !strings.Contains(opi, `"name": "precision"`) || !strings.Contains(opi, `"default": 0.75`) {
		t.Errorf("expected OpenAPI default for precision, got:\n%s", opi)
	}
	if !strings.Contains(opi, `"name": "active"`) || !strings.Contains(opi, `"default": true`) {
		t.Errorf("expected OpenAPI default for active, got:\n%s", opi)
	}
	if !strings.Contains(opi, `"name": "name"`) || !strings.Contains(opi, `"default": "guest"`) {
		t.Errorf("expected OpenAPI default for name, got:\n%s", opi)
	}
}

func TestGenerateEmbeddedStructBindings(t *testing.T) {
	g := &meta{
		APIs: []metaAPI{{
			FuncName:   "UserDetail",
			ParamsType: "UserDetailParams",
			Method:     "GET",
			Path:       "/users/:id",
		}},
		structs: map[string]*ast.StructType{
			"CommonParams": {
				Fields: &ast.FieldList{List: []*ast.Field{
					{Names: []*ast.Ident{{Name: "RequestID"}}, Tag: &ast.BasicLit{Value: "`" + _TAG_HEADER + ":\"X-Request-ID\"`"}},
				}},
			},
			"UserDetailParams": {
				Fields: &ast.FieldList{List: []*ast.Field{
					{Type: &ast.Ident{Name: "CommonParams"}},
					{Names: []*ast.Ident{{Name: "ID"}}, Tag: &ast.BasicLit{Value: "`" + _TAG_URL + ":\"id\"`"}},
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
	if !strings.Contains(code, `req.RequestID = c.Get("X-Request-ID")`) {
		t.Errorf("expected embedded header binding, got:\n%s", code)
	}
	if !strings.Contains(code, `req.ID = c.Params("id")`) {
		t.Errorf("expected embedded URL binding, got:\n%s", code)
	}
}

func TestCollectFieldsWithSelectorEmbeddedStruct(t *testing.T) {
	m := &meta{
		structs: map[string]*ast.StructType{
			"UpdateUser": {
				Fields: &ast.FieldList{List: []*ast.Field{
					{Type: &ast.SelectorExpr{X: &ast.Ident{Name: "request"}, Sel: &ast.Ident{Name: "ReqFirstIndex"}}},
				}},
			},
			"ReqFirstIndex": {
				Fields: &ast.FieldList{List: []*ast.Field{
					{Names: []*ast.Ident{{Name: "KtLimit"}}, Type: &ast.Ident{Name: "int"}, Tag: &ast.BasicLit{Value: "`" + _TAG_QUERY + ":\"limit,default=128\"`"}},
					{Names: []*ast.Ident{{Name: "KtToken"}}, Type: &ast.Ident{Name: "string"}, Tag: &ast.BasicLit{Value: "`" + _TAG_HEADER + ":\"KT_TOKEN\"`"}},
					{Names: []*ast.Ident{{Name: "FirstIndex"}}, Type: &ast.Ident{Name: "string"}, Tag: &ast.BasicLit{Value: "`" + _TAG_URL + ":\"first_id\"`"}},
				}},
			},
		},
	}

	update := m.structs["UpdateUser"]
	fields := m.collectFields(update)
	if len(fields) != 3 {
		t.Fatalf("expected 3 fields from selector embedded struct, got %d", len(fields))
	}
	wantNames := []string{"KtLimit", "KtToken", "FirstIndex"}
	for i, f := range fields {
		if len(f.Names) == 0 || f.Names[0].Name != wantNames[i] {
			t.Fatalf("unexpected field at %d: %#v", i, f)
		}
		if f.Tag == nil {
			t.Fatalf("field %s has nil tag", f.Names[0].Name)
		}
		trimmed := strings.Trim(f.Tag.Value, "`")
		stag := reflect.StructTag(trimmed)
		switch wantNames[i] {
		case "KtLimit":
			if val, ok := stag.Lookup(_TAG_QUERY); !ok || val != "limit,default=128" {
				t.Fatalf("expected query tag for KtLimit, tag=%q", trimmed)
			}
		case "KtToken":
			if val, ok := stag.Lookup(_TAG_HEADER); !ok || val != "KT_TOKEN" {
				t.Fatalf("expected header tag for KtToken, tag=%q", trimmed)
			}
		case "FirstIndex":
			if val, ok := stag.Lookup(_TAG_URL); !ok || val != "first_id" {
				t.Fatalf("expected url tag for FirstIndex, tag=%q", trimmed)
			}
		}
	}
}

func TestGenerateNoDuplicateBindingsForNestedEmbeds(t *testing.T) {
	g := &meta{
		APIs: []metaAPI{{
			FuncName:   "userUpdate",
			ParamsType: "UpdateUser",
			Method:     "PATCH",
			Path:       "/user/:first_id",
		}},
		structs: map[string]*ast.StructType{
			"UpdateUser": {
				Fields: &ast.FieldList{List: []*ast.Field{
					{Type: &ast.SelectorExpr{X: &ast.Ident{Name: "request"}, Sel: &ast.Ident{Name: "ReqFirstIndex"}}},
					{Type: &ast.Ident{Name: "RegisterUser"}},
				}},
			},
			"RegisterUser": {
				Fields: &ast.FieldList{List: []*ast.Field{
					{Type: &ast.SelectorExpr{X: &ast.Ident{Name: "request"}, Sel: &ast.Ident{Name: "ReqFirstIndex"}}},
				}},
			},
			"ReqFirstIndex": {
				Fields: &ast.FieldList{List: []*ast.Field{
					{Names: []*ast.Ident{{Name: "KtLimit"}}, Type: &ast.Ident{Name: "int"}, Tag: &ast.BasicLit{Value: "`" + _TAG_QUERY + ":\"limit,default=128\"`"}},
					{Names: []*ast.Ident{{Name: "KtToken"}}, Type: &ast.Ident{Name: "string"}, Tag: &ast.BasicLit{Value: "`" + _TAG_HEADER + ":\"KT_TOKEN\"`"}},
					{Names: []*ast.Ident{{Name: "FirstIndex"}}, Type: &ast.Ident{Name: "string"}, Tag: &ast.BasicLit{Value: "`" + _TAG_URL + ":\"first_id\"`"}},
				}},
			},
		},
		LibName: "lib",
	}

	src, err := g.generateWithTemplates()
	if err != nil {
		t.Fatal(err)
	}
	if len(g.APIs[0].Binds) != 3 {
		t.Fatalf("expected 3 binds, got %d", len(g.APIs[0].Binds))
	}
	code := string(src)
	if strings.Count(code, `c.QueryInt("limit", 128)`) != 1 {
		t.Fatalf("expected single query binding, got:\n%s", code)
	}
	if strings.Count(code, `c.Get("KT_TOKEN")`) != 1 {
		t.Fatalf("expected single header binding, got:\n%s", code)
	}
	if strings.Count(code, `c.Params("first_id")`) != 1 {
		t.Fatalf("expected single url binding, got:\n%s", code)
	}
}

func TestGenerateBodyParserForPost(t *testing.T) {
	g := &meta{
		APIs: []metaAPI{{
			FuncName:   "CreateUser",
			ParamsType: "CreateUserParams",
			Method:     "POST",
			Path:       "/users",
		}},
		structs: map[string]*ast.StructType{
			"CreateUserParams": {
				Fields: &ast.FieldList{List: []*ast.Field{
					{Names: []*ast.Ident{{Name: "Email"}}, Type: &ast.Ident{Name: "string"}},
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
	if !strings.Contains(code, `if err := c.BodyParser(req); err != nil {`) {
		t.Errorf("expected BodyParser call for POST, got:\n%s", code)
	}
	openapi, err := g.generateOpenAPIWithTemplates()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(openapi), `"requestBody"`) {
		t.Errorf("expected requestBody in OpenAPI for POST, got:\n%s", string(openapi))
	}
}
