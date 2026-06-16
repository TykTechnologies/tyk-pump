package main

import (
	"os"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

// goModDeclaredVersion parses the `go` directive (major, minor) from the module's
// go.mod. It is the single source of truth SYS-REQ-024 constrains.
func goModDeclaredVersion(t *testing.T) (major, minor int) {
	t.Helper()
	data, err := os.ReadFile("go.mod")
	if err != nil {
		t.Fatalf("read go.mod: %v", err)
	}
	re := regexp.MustCompile(`(?m)^go\s+(\d+)\.(\d+)(?:\.\d+)?\s*$`)
	m := re.FindStringSubmatch(string(data))
	if m == nil {
		t.Fatalf("go.mod has no parseable `go` directive")
	}
	major, err = strconv.Atoi(m[1])
	if err != nil {
		t.Fatalf("parse go major version %q: %v", m[1], err)
	}
	minor, err = strconv.Atoi(m[2])
	if err != nil {
		t.Fatalf("parse go minor version %q: %v", m[2], err)
	}
	return major, minor
}

// goVersionAtLeast125 is the boolean predicate SYS-REQ-024 formalizes:
// go_125_or_later. It is true iff the declared toolchain is >= 1.25.
func goVersionAtLeast125(major, minor int) bool {
	if major != 1 {
		return major > 1
	}
	return minor >= 25
}

// Verifies: SYS-REQ-024
// SYS-REQ-024:nominal:nominal
//
// MCDC SYS-REQ-024: go_125_or_later=T => TRUE
//
// SYS-REQ-024 is the build invariant "the tyk-pump Go module shall be built
// with Go toolchain version 1.25 or later, as declared in go.mod". The
// requirement's single boolean condition (go_125_or_later) is observable in
// pure Go by reading the module's own go.mod `go` directive and applying the
// >= 1.25 predicate. This test drives the satisfied (TRUE) row by asserting the
// declared version is at least 1.25.
//
// The FALSE row (go_125_or_later=F => FALSE) is the violation the released
// module never ships: a go.mod declaring < 1.25 would fail this very assertion
// in CI before release, so there is no honest runtime witness for it here — it
// is structurally prevented by the gate rather than produced by correct code.
// The predicate's F-arm is exercised directly below over a synthetic version
// pair so the classification logic itself is proven, without fabricating a
// build state the module does not have.
//
//mcdc:ignore SYS-REQ-024: go_125_or_later=F => FALSE — go.mod pins `go 1.25.0`; the Go toolchain refuses to build a module whose declared `go` directive exceeds the running toolchain, so any environment with a sub-1.25 toolchain cannot produce a tyk-pump binary at all, and CI builds/this very assertion reject a go.mod regressed below 1.25 before release. The go_125_or_later=F build state is therefore structurally unreachable in a correct build rather than produced by correct code. [reviewed: human:leo]
func TestBuildInvariant_GoToolchainAtLeast125_SYSREQ024(t *testing.T) {
	major, minor := goModDeclaredVersion(t)
	if !goVersionAtLeast125(major, minor) {
		t.Fatalf("go.mod declares go %d.%d; SYS-REQ-024 requires >= 1.25", major, minor)
	}

	// Prove the predicate's independent effect (both arms) so the >= 1.25
	// classification is trustworthy, not just the current toolchain value.
	if goVersionAtLeast125(1, 24) {
		t.Fatal("go 1.24 must classify as NOT go_125_or_later")
	}
	if !goVersionAtLeast125(1, 25) {
		t.Fatal("go 1.25 must classify as go_125_or_later")
	}
	if !goVersionAtLeast125(2, 0) {
		t.Fatal("go 2.0 must classify as go_125_or_later")
	}
}

// TestBuildInvariant_GoModToolchainStringSanity guards the parser against a
// go.mod shape it cannot read, keeping the SYS-REQ-024 witness honest if the
// `go` directive format ever changes.
//
// Verifies: SYS-REQ-024
func TestBuildInvariant_GoModToolchainStringSanity(t *testing.T) {
	data, err := os.ReadFile("go.mod")
	if err != nil {
		t.Fatalf("read go.mod: %v", err)
	}
	if !strings.Contains(string(data), "\ngo ") && !strings.HasPrefix(string(data), "go ") {
		t.Fatal("go.mod must contain a `go` directive for SYS-REQ-024 to be verifiable")
	}
}
