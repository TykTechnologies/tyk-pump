package server

import (
	"bytes"
	"context"
	"go/ast"
	"go/parser"
	"go/printer"
	"go/token"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"testing"
	"time"
)

// SW-REQ-032:nominal:boundary
func TestResolveHealthCheckParams_AllBranches(t *testing.T) {
	cases := []struct {
		name     string
		inEnd    string
		inPort   int
		wantEnd  string
		wantPort int
	}{
		{"both defaults", "", 0, defaultHealthEndpoint, defaultHealthPort},
		{"endpoint configured", "ping", 0, "ping", defaultHealthPort},
		{"port configured", "", 9000, defaultHealthEndpoint, 9000},
		{"both configured", "alive", 9001, "alive", 9001},
	}
	for _, c := range cases {
		gotE, gotP := resolveHealthCheckParams(c.inEnd, c.inPort)
		if gotE != c.wantEnd || gotP != c.wantPort {
			t.Fatalf("%s: got (%q,%d) want (%q,%d)", c.name, gotE, gotP, c.wantEnd, c.wantPort)
		}
	}
}

// Verifies: SW-REQ-032
// MCDC SW-REQ-032: enable_profiling=F, pprof_route_registered=F => TRUE
func TestBuildHealthCheckRouter_RegistersExpectedRoutes(t *testing.T) {
	// health-only (no profiling): /<endpoint> registered, /debug/pprof/* not
	r := buildHealthCheckRouter("alive", false)
	if r == nil {
		t.Fatal("router is nil")
	}
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/alive", nil)
	r.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("/alive returned %d, want 200", rec.Code)
	}
	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/debug/pprof/", nil)
	r.ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("/debug/pprof/ without profiling returned %d, want 404", rec.Code)
	}
}

// Verifies: SW-REQ-032
// MCDC SW-REQ-032: enable_profiling=T, pprof_route_registered=T => TRUE
func TestBuildHealthCheckRouter_RegistersPprofWhenEnabled(t *testing.T) {
	// With profiling enabled, /debug/pprof/heap should route through the pprof handler
	// (which serves the heap profile inline). A non-404 response confirms registration.
	r := buildHealthCheckRouter("health", true)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/debug/pprof/heap?debug=1", nil)
	r.ServeHTTP(rec, req)
	if rec.Code == http.StatusNotFound {
		t.Fatalf("/debug/pprof/heap returned 404 with profiling enabled; pprof catchall not registered")
	}
}

// SW-REQ-032:listener_bind_scope_external:nominal
// SW-REQ-032:listener_bind_scope_external:negative
// SW-REQ-032:listener_bind_scope_external:review
func TestServeHealthCheck_BindsExternalInterface(t *testing.T) {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "server.go", nil, 0)
	if err != nil {
		t.Fatal(err)
	}

	var listenAddr ast.Expr
	ast.Inspect(file, func(n ast.Node) bool {
		fn, ok := n.(*ast.FuncDecl)
		if !ok || fn.Name.Name != "ServeHealthCheck" {
			return true
		}
		ast.Inspect(fn.Body, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok || len(call.Args) == 0 {
				return true
			}
			sel, ok := call.Fun.(*ast.SelectorExpr)
			if !ok || sel.Sel.Name != "ListenAndServe" {
				return true
			}
			pkg, ok := sel.X.(*ast.Ident)
			if !ok || pkg.Name != "http" {
				return true
			}
			listenAddr = call.Args[0]
			return false
		})
		return false
	})
	if listenAddr == nil {
		t.Fatal("ServeHealthCheck does not call http.ListenAndServe")
	}
	if expressionContainsLoopbackHost(listenAddr) {
		t.Fatalf("health endpoint listener is restricted to loopback: %s", expressionString(listenAddr))
	}

	bin, ok := listenAddr.(*ast.BinaryExpr)
	if !ok || bin.Op != token.ADD {
		t.Fatalf("health endpoint listener address has unexpected shape: %s", expressionString(listenAddr))
	}
	lit, ok := bin.X.(*ast.BasicLit)
	if !ok || lit.Value != "\":\"" {
		t.Fatalf("health endpoint listener should bind wildcard host via \":\" prefix, got %s", expressionString(bin.X))
	}
	call, ok := bin.Y.(*ast.CallExpr)
	if !ok {
		t.Fatalf("health endpoint listener should append the resolved port, got %s", expressionString(bin.Y))
	}
	sel, ok := call.Fun.(*ast.SelectorExpr)
	pkg, pkgOK := sel.X.(*ast.Ident)
	if !ok || !pkgOK || pkg.Name != "fmt" || sel.Sel.Name != "Sprint" {
		t.Fatalf("health endpoint listener should append fmt.Sprint(port), got %s", expressionString(bin.Y))
	}
	if len(call.Args) != 1 {
		t.Fatalf("fmt.Sprint should receive one port argument, got %d", len(call.Args))
	}
	portIdent, ok := call.Args[0].(*ast.Ident)
	if !ok || portIdent.Name != "port" {
		t.Fatalf("health endpoint listener should use resolved port, got %s", expressionString(call.Args[0]))
	}
}

// Verifies: SW-REQ-032
// Verifies: SYS-REQ-012
// Verifies: KI:health-listener-bind-failure-logfatal
// Reproduces: health-listener-bind-failure-logfatal
func TestServeHealthCheckOccupiedPortFatal_KI(t *testing.T) {
	if os.Getenv("TYK_PUMP_HEALTH_BIND_FATAL_CHILD") == "1" {
		port, err := strconv.Atoi(os.Getenv("TYK_PUMP_HEALTH_OCCUPIED_PORT"))
		if err != nil {
			t.Fatalf("invalid occupied port: %v", err)
		}
		ServeHealthCheck("health", port, false)
		t.Fatal("ServeHealthCheck returned after occupied listener port")
	}

	listener, err := net.Listen("tcp", ":0")
	if err != nil {
		t.Fatalf("bind occupied-port listener: %v", err)
	}
	defer listener.Close()

	tcpAddr, ok := listener.Addr().(*net.TCPAddr)
	if !ok {
		t.Fatalf("listener address has unexpected type %T", listener.Addr())
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, os.Args[0], "-test.run", "^TestServeHealthCheckOccupiedPortFatal_KI$")
	cmd.Env = append(os.Environ(),
		"TYK_PUMP_HEALTH_BIND_FATAL_CHILD=1",
		"TYK_PUMP_HEALTH_OCCUPIED_PORT="+strconv.Itoa(tcpAddr.Port),
	)
	output, err := cmd.CombinedOutput()
	if ctx.Err() == context.DeadlineExceeded {
		t.Fatalf("child process hung waiting for occupied-port fatal; output:\n%s", output)
	}
	if err == nil {
		t.Fatalf("child process exited successfully; expected log.Fatal on occupied port; output:\n%s", output)
	}
	if !strings.Contains(string(output), "Error serving health check endpoint") {
		t.Fatalf("child output did not include health listener fatal message; output:\n%s", output)
	}
}

func expressionContainsLoopbackHost(expr ast.Expr) bool {
	found := false
	ast.Inspect(expr, func(n ast.Node) bool {
		if found {
			return false
		}
		lit, ok := n.(*ast.BasicLit)
		if !ok {
			return true
		}
		value := strings.ToLower(lit.Value)
		if strings.Contains(value, "localhost") || strings.Contains(value, "127.0.0.1") || strings.Contains(value, "[::1]") {
			found = true
			return false
		}
		return true
	})
	return found
}

func expressionString(expr ast.Expr) string {
	if expr == nil {
		return "<nil>"
	}
	var buf bytes.Buffer
	if err := printer.Fprint(&buf, token.NewFileSet(), expr); err != nil {
		return "<expr>"
	}
	return buf.String()
}
