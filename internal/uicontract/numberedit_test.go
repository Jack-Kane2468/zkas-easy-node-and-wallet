// Walk's NumberEdit.Value is an interface property. Its runtime setter accepts
// float64 only; an int literal silently falls back to zero and can abort startup.
package uicontract

import (
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"testing"
)

func TestNumberEditLiteralDefaultsAreFloat64(t *testing.T) {
	files, e := filepath.Glob("../../cmd/zkas-manager/*windows.go")
	if e != nil || len(files) == 0 {
		t.Fatal("manager source not found", e)
	}
	set := token.NewFileSet()
	for _, path := range files {
		file, e := parser.ParseFile(set, path, nil, 0)
		if e != nil {
			t.Fatal(e)
		}
		ast.Inspect(file, func(n ast.Node) bool {
			lit, ok := n.(*ast.CompositeLit)
			if !ok {
				return true
			}
			id, ok := lit.Type.(*ast.Ident)
			if !ok || id.Name != "NumberEdit" {
				return true
			}
			for _, field := range lit.Elts {
				kv, ok := field.(*ast.KeyValueExpr)
				if !ok {
					continue
				}
				key, ok := kv.Key.(*ast.Ident)
				if !ok || key.Name != "Value" {
					continue
				}
				if value, ok := kv.Value.(*ast.BasicLit); ok && value.Kind != token.FLOAT {
					t.Errorf("%s: NumberEdit.Value literal must be float64, not %s (Walk otherwise substitutes zero)", set.Position(value.Pos()), value.Kind)
				}
			}
			return true
		})
	}
}
