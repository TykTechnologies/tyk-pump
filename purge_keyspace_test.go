package main

import (
	"go/ast"
	"go/parser"
	"go/token"
	"testing"
)

// SW-REQ-001:wire_format_suffix_dispatch:nominal
// SW-REQ-001:wire_format_suffix_dispatch:negative
// SW-REQ-001:wire_format_suffix_dispatch:review
func TestStartPurgeLoopDrainsEverySerializerSuffixForEveryAnalyticsKey(t *testing.T) {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "main.go", nil, 0)
	if err != nil {
		t.Fatalf("parse main.go: %v", err)
	}

	fn := findFuncDecl(file, "StartPurgeLoop")
	if fn == nil {
		t.Fatal("StartPurgeLoop not found")
	}

	var hasShardLoop bool
	var hasSerializerLoop bool
	var suffixAppliedToAnalyticsKey bool
	var consumesSuffixedKey bool
	var dispatchesWithMatchingSerializer bool

	ast.Inspect(fn.Body, func(n ast.Node) bool {
		switch node := n.(type) {
		case *ast.ForStmt:
			if isAnalyticsShardLoop(node) {
				hasShardLoop = true
			}
		case *ast.RangeStmt:
			if identName(node.Value) == "serializerMethod" && exprString(node.X) == "AnalyticsSerializers" {
				hasSerializerLoop = true
			}
		case *ast.AssignStmt:
			if assignsAnalyticsKeySuffix(node) {
				suffixAppliedToAnalyticsKey = true
			}
		case *ast.CallExpr:
			if isGetAndDeleteSetOnAnalyticsKey(node) {
				consumesSuffixedKey = true
			}
			if isPreprocessCallWithSerializerAndKey(node) {
				dispatchesWithMatchingSerializer = true
			}
		}
		return true
	})

	if !hasShardLoop {
		t.Fatal("StartPurgeLoop must iterate the legacy analytics key plus shard keys _0.._9")
	}
	if !hasSerializerLoop {
		t.Fatal("StartPurgeLoop must range over AnalyticsSerializers for each analytics key")
	}
	if !suffixAppliedToAnalyticsKey {
		t.Fatal("StartPurgeLoop must append serializerMethod.GetSuffix() to analyticsKeyName before consuming records")
	}
	if !consumesSuffixedKey {
		t.Fatal("StartPurgeLoop must call AnalyticsStore.GetAndDeleteSet with the suffixed analyticsKeyName")
	}
	if !dispatchesWithMatchingSerializer {
		t.Fatal("StartPurgeLoop must pass the matching serializerMethod and analyticsKeyName to PreprocessAnalyticsValues")
	}
}

func findFuncDecl(file *ast.File, name string) *ast.FuncDecl {
	for _, decl := range file.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if ok && fn.Name.Name == name {
			return fn
		}
	}
	return nil
}

func isAnalyticsShardLoop(loop *ast.ForStmt) bool {
	init, ok := loop.Init.(*ast.AssignStmt)
	if !ok || len(init.Lhs) != 1 || identName(init.Lhs[0]) != "i" {
		return false
	}
	if len(init.Rhs) != 1 || !isNegativeOne(init.Rhs[0]) {
		return false
	}

	cond, ok := loop.Cond.(*ast.BinaryExpr)
	if !ok || cond.Op.String() != "<" || identName(cond.X) != "i" {
		return false
	}
	lit, ok := cond.Y.(*ast.BasicLit)
	return ok && lit.Value == "10"
}

func isNegativeOne(expr ast.Expr) bool {
	unary, ok := expr.(*ast.UnaryExpr)
	if !ok || unary.Op.String() != "-" {
		return false
	}
	lit, ok := unary.X.(*ast.BasicLit)
	return ok && lit.Value == "1"
}

func assignsAnalyticsKeySuffix(assign *ast.AssignStmt) bool {
	if assign.Tok.String() != "+=" || len(assign.Lhs) != 1 || identName(assign.Lhs[0]) != "analyticsKeyName" {
		return false
	}
	call, ok := assign.Rhs[0].(*ast.CallExpr)
	if !ok {
		return false
	}
	sel, ok := call.Fun.(*ast.SelectorExpr)
	return ok && identName(sel.X) == "serializerMethod" && sel.Sel.Name == "GetSuffix"
}

func isGetAndDeleteSetOnAnalyticsKey(call *ast.CallExpr) bool {
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok || sel.Sel.Name != "GetAndDeleteSet" || identName(sel.X) != "AnalyticsStore" {
		return false
	}
	return len(call.Args) > 0 && identName(call.Args[0]) == "analyticsKeyName"
}

func isPreprocessCallWithSerializerAndKey(call *ast.CallExpr) bool {
	if identName(call.Fun) != "PreprocessAnalyticsValues" || len(call.Args) < 4 {
		return false
	}
	return identName(call.Args[1]) == "serializerMethod" && identName(call.Args[2]) == "analyticsKeyName"
}

func identName(expr ast.Expr) string {
	if ident, ok := expr.(*ast.Ident); ok {
		return ident.Name
	}
	return ""
}

func exprString(expr ast.Expr) string {
	return identName(expr)
}
