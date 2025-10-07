package generator

import (
	"strings"
	"testing"
)

func TestDefaultLiteral(t *testing.T) {
	if defaultLiteral("10", kindInt) != "10" {
		t.Fatal("int default")
	}
	if defaultLiteral("3.14", kindFloat) != "3.14" {
		t.Fatal("float default")
	}
	if defaultLiteral("true", kindBool) != "true" {
		t.Fatal("bool default")
	}
	if defaultLiteral("x", kindString) != `"x"` {
		t.Fatal("string default")
	}
}

func TestBuildBindCode_Simple(t *testing.T) {
	code := BuildBindCode([]FieldBind{
		{Name: "ID", Kind: "url", Key: "id"},
		{Name: "Token", Kind: "header", Key: "X-Token"},
		{Name: "Limit", Kind: "query", Key: "limit", QueryFunc: "QueryInt", HasDefault: true, DefaultValue: "10", KindHint: kindInt},
		{Name: "Enable", Kind: "query", Key: "enable", QueryFunc: "QueryBool", HasDefault: true, DefaultValue: "true", KindHint: kindBool},
		{Name: "Session", Kind: "cookie", Key: "session"},
	})

	want := []string{
		`req.ID = c.Params("id")`,
		`req.Token = c.Get("X-Token")`,
		`req.Limit = c.QueryInt("limit", 10)`,
		`req.Enable = c.QueryBool("enable", true)`,
		`req.Session = c.Cookies("session")`,
	}
	for _, w := range want {
		if !strings.Contains(code, w) {
			t.Fatalf("missing line: %s\ncode:\n%s", w, code)
		}
	}
}
