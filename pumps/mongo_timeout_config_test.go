package pumps

import (
	"go/ast"
	"go/parser"
	"go/token"
	"testing"

	"github.com/stretchr/testify/require"
)

// SW-REQ-034:backend_connection_timeout_propagated:nominal
// SW-REQ-034:backend_connection_timeout_propagated:negative
// SW-REQ-034:backend_connection_timeout_propagated:review
// SW-REQ-035:backend_connection_timeout_propagated:nominal
// SW-REQ-035:backend_connection_timeout_propagated:negative
// SW-REQ-035:backend_connection_timeout_propagated:review
// SW-REQ-036:backend_connection_timeout_propagated:nominal
// SW-REQ-036:backend_connection_timeout_propagated:negative
// SW-REQ-036:backend_connection_timeout_propagated:review
func TestMongoPumpsPropagateConfiguredConnectionTimeout(t *testing.T) {
	for _, tc := range []struct {
		name     string
		file     string
		receiver string
	}{
		{name: "standard", file: "mongo.go", receiver: "MongoPump"},
		{name: "selective", file: "mongo_selective.go", receiver: "MongoSelectivePump"},
		{name: "aggregate", file: "mongo_aggregate.go", receiver: "MongoAggregatePump"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			file := parseGoFile(t, tc.file)
			fn := findMethod(t, file, tc.receiver, "connect")
			require.True(t, hasPersistentClientTimeoutFromReceiver(fn), "%s connect must set persistent.ClientOpts.ConnectionTimeout from m.timeout", tc.receiver)
		})
	}
}

// SW-REQ-034:backend_connection_timeout_propagated:nominal
// SW-REQ-035:backend_connection_timeout_propagated:nominal
// SW-REQ-036:backend_connection_timeout_propagated:nominal
func TestInitialisePumpsAppliesTimeoutBeforeInit(t *testing.T) {
	file := parseGoFile(t, "../main.go")
	fn := findFunction(t, file, "initialisePumps")

	setTimeoutPos := token.NoPos
	initPos := token.NoPos
	setTimeoutUsesConfiguredValue := false
	initUsesMeta := false
	ast.Inspect(fn.Body, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		sel, ok := call.Fun.(*ast.SelectorExpr)
		if !ok {
			return true
		}
		if isIdent(sel.X, "thisPmp") && sel.Sel.Name == "SetTimeout" && setTimeoutPos == token.NoPos {
			setTimeoutPos = call.Pos()
			setTimeoutUsesConfiguredValue = len(call.Args) == 1 && isSelector(call.Args[0], "pmp", "Timeout")
		}
		if isIdent(sel.X, "thisPmp") && sel.Sel.Name == "Init" && initPos == token.NoPos {
			initPos = call.Pos()
			initUsesMeta = len(call.Args) == 1 && isSelector(call.Args[0], "pmp", "Meta")
		}
		return true
	})

	require.NotEqual(t, token.NoPos, setTimeoutPos, "initialisePumps must call thisPmp.SetTimeout")
	require.NotEqual(t, token.NoPos, initPos, "initialisePumps must call thisPmp.Init")
	require.True(t, setTimeoutUsesConfiguredValue, "initialisePumps must pass pmp.Timeout into thisPmp.SetTimeout")
	require.True(t, initUsesMeta, "initialisePumps must pass pmp.Meta into thisPmp.Init")
	require.Less(t, int(setTimeoutPos), int(initPos), "initialisePumps must apply timeout before Init builds backend clients")
}

// KI tripwire for SW-REQ-034; not requirement-success evidence.
// Verifies: KI:mongo-standard-insert-error-double-send-goroutine-leak
// Reproduces: mongo-standard-insert-error-double-send-goroutine-leak
func TestMongoPump_WriteData_InsertErrDoubleSend_KI(t *testing.T) {
	file := parseGoFile(t, "mongo.go")
	fn := findMethod(t, file, "MongoPump", "WriteData")

	require.True(t, hasInsertErrDoubleSendPattern(fn),
		"KI active: insert goroutine sends errCh <- err and then falls through to an unconditional errCh <- nil")
	require.True(t, returnsOnFirstErrChError(fn),
		"KI active: WriteData returns as soon as it reads the first non-nil errCh value")
}

func parseGoFile(t *testing.T, path string) *ast.File {
	t.Helper()
	file, err := parser.ParseFile(token.NewFileSet(), path, nil, 0)
	require.NoError(t, err)
	return file
}

func findFunction(t *testing.T, file *ast.File, name string) *ast.FuncDecl {
	t.Helper()
	for _, decl := range file.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if ok && fn.Recv == nil && fn.Name.Name == name {
			return fn
		}
	}
	t.Fatalf("function %s not found", name)
	return nil
}

func findMethod(t *testing.T, file *ast.File, receiverName, methodName string) *ast.FuncDecl {
	t.Helper()
	for _, decl := range file.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Recv == nil || fn.Name.Name != methodName {
			continue
		}
		if receiverTypeName(fn.Recv.List[0].Type) == receiverName {
			return fn
		}
	}
	t.Fatalf("method %s.%s not found", receiverName, methodName)
	return nil
}

func receiverTypeName(expr ast.Expr) string {
	switch x := expr.(type) {
	case *ast.Ident:
		return x.Name
	case *ast.StarExpr:
		return receiverTypeName(x.X)
	default:
		return ""
	}
}

func hasPersistentClientTimeoutFromReceiver(fn *ast.FuncDecl) bool {
	found := false
	ast.Inspect(fn.Body, func(n ast.Node) bool {
		lit, ok := n.(*ast.CompositeLit)
		if !ok || !isPersistentClientOpts(lit.Type) {
			return true
		}
		for _, elt := range lit.Elts {
			kv, ok := elt.(*ast.KeyValueExpr)
			if ok && isIdent(kv.Key, "ConnectionTimeout") && isSelector(kv.Value, "m", "timeout") {
				found = true
				return false
			}
		}
		return true
	})
	return found
}

func isPersistentClientOpts(expr ast.Expr) bool {
	sel, ok := expr.(*ast.SelectorExpr)
	return ok && isIdent(sel.X, "persistent") && sel.Sel.Name == "ClientOpts"
}

func isSelector(expr ast.Expr, objectName, fieldName string) bool {
	sel, ok := expr.(*ast.SelectorExpr)
	return ok && isIdent(sel.X, objectName) && sel.Sel.Name == fieldName
}

func isIdent(expr ast.Expr, name string) bool {
	ident, ok := expr.(*ast.Ident)
	return ok && ident.Name == name
}

func hasInsertErrDoubleSendPattern(fn *ast.FuncDecl) bool {
	found := false
	ast.Inspect(fn.Body, func(n ast.Node) bool {
		lit, ok := n.(*ast.FuncLit)
		if !ok {
			return true
		}

		errChannelNames := channelParamNames(lit)
		errChannelNames["errCh"] = true
		errSendInBranch := false
		nilSendAfterBranch := false
		for _, stmt := range lit.Body.List {
			if stmtContainsIfWithSend(stmt, errChannelNames, "err") {
				errSendInBranch = true
				continue
			}
			if errSendInBranch && stmtContainsSend(stmt, errChannelNames, "nil") {
				nilSendAfterBranch = true
			}
		}

		if errSendInBranch && nilSendAfterBranch {
			found = true
			return false
		}
		return true
	})
	return found
}

func returnsOnFirstErrChError(fn *ast.FuncDecl) bool {
	found := false
	ast.Inspect(fn.Body, func(n ast.Node) bool {
		caseClause, ok := n.(*ast.CommClause)
		if !ok || caseClause.Comm == nil {
			return true
		}
		comm, ok := caseClause.Comm.(*ast.AssignStmt)
		if !ok || len(comm.Rhs) != 1 {
			return true
		}
		recv, ok := comm.Rhs[0].(*ast.UnaryExpr)
		if !ok || recv.Op != token.ARROW || !isIdent(recv.X, "errCh") {
			return true
		}
		for _, stmt := range caseClause.Body {
			if stmtContainsIfWithReturn(stmt, "err") {
				found = true
				return false
			}
		}
		return true
	})
	return found
}

func isErrNotNil(expr ast.Expr) bool {
	bin, ok := expr.(*ast.BinaryExpr)
	return ok && bin.Op == token.NEQ && isIdent(bin.X, "err") && isIdent(bin.Y, "nil")
}

func channelParamNames(lit *ast.FuncLit) map[string]bool {
	names := map[string]bool{}
	if lit.Type.Params == nil {
		return names
	}
	for _, field := range lit.Type.Params.List {
		if !isChanOfError(field.Type) {
			continue
		}
		for _, name := range field.Names {
			names[name.Name] = true
		}
	}
	return names
}

func isChanOfError(expr ast.Expr) bool {
	ch, ok := expr.(*ast.ChanType)
	return ok && isIdent(ch.Value, "error")
}

func stmtContainsIfWithSend(stmt ast.Stmt, channelNames map[string]bool, valueName string) bool {
	found := false
	ast.Inspect(stmt, func(n ast.Node) bool {
		if found {
			return false
		}
		ifStmt, ok := n.(*ast.IfStmt)
		if !ok {
			return true
		}
		found = blockContainsSend(ifStmt.Body, channelNames, valueName)
		return !found
	})
	return found
}

func stmtContainsSend(stmt ast.Stmt, channelNames map[string]bool, valueName string) bool {
	found := false
	ast.Inspect(stmt, func(n ast.Node) bool {
		send, ok := n.(*ast.SendStmt)
		if ok && isChannelName(send.Chan, channelNames) && isIdent(send.Value, valueName) {
			found = true
			return false
		}
		return true
	})
	return found
}

func blockContainsSend(block *ast.BlockStmt, channelNames map[string]bool, valueName string) bool {
	for _, stmt := range block.List {
		if stmtContainsSend(stmt, channelNames, valueName) {
			return true
		}
	}
	return false
}

func isChannelName(expr ast.Expr, channelNames map[string]bool) bool {
	ident, ok := expr.(*ast.Ident)
	return ok && channelNames[ident.Name]
}

func stmtContainsIfWithReturn(stmt ast.Stmt, name string) bool {
	found := false
	ast.Inspect(stmt, func(n ast.Node) bool {
		if found {
			return false
		}
		ifStmt, ok := n.(*ast.IfStmt)
		if !ok {
			return true
		}
		found = blockContainsReturn(ifStmt.Body, name)
		return !found
	})
	return found
}

func blockContainsReturn(block *ast.BlockStmt, name string) bool {
	found := false
	ast.Inspect(block, func(n ast.Node) bool {
		ret, ok := n.(*ast.ReturnStmt)
		if ok && len(ret.Results) == 1 && isIdent(ret.Results[0], name) {
			found = true
			return false
		}
		return true
	})
	return found
}
