package main

import (
	"os"
	"regexp"
	"strings"
	"testing"
)

// Verifies: SYS-REQ-026
// SYS-REQ-026:nominal:positive
//
// MCDC SYS-REQ-026: fips_build_available=T => TRUE
//
// SYS-REQ-026 constrains the build to make a FIPS-compliant binary available
// "via 'make build-fips', producing a binary with the boringcrypto
// GOEXPERIMENT." The availability of that build is a property of the Makefile
// the requirement names. We witness the satisfied (TRUE) row by asserting the
// `build-fips` target exists and is wired to boringcrypto exactly as the
// requirement and its rationale require:
//
//   - a `build-fips:` target is declared, and
//   - its recipe enables the boringcrypto GOEXPERIMENT and build tag.
//
// This is the same inspection-of-the-named-artifact discharge used for
// SYS-REQ-024 (go.mod): the requirement is about what the build system makes
// available, so reading that build system is the honest witness. It does not
// invoke the FIPS toolchain itself (which requires a boringcrypto-capable Go
// toolchain in the build environment, not a unit-test concern).
//
// The FALSE row (fips_build_available=F => FALSE) is the violation the shipped
// repository never produces: removing or de-wiring the build-fips target would
// fail this assertion in CI before release, so it is structurally prevented
// rather than produced by correct code and has no honest runtime witness here.
//
//mcdc:ignore SYS-REQ-026: fips_build_available=F => FALSE — the requirement is discharged by inspecting the named build artifact (the `build-fips` Makefile target wired to GOEXPERIMENT=boringcrypto). That target is committed in the repository, and this assertion plus CI fail before release if it is removed or de-wired, so a shipped tree with fips_build_available=F cannot exist. The violation is structurally prevented by the committed build system rather than produced by correct code. [reviewed: human:leo]
func TestBuildInvariant_FIPSBuildAvailable_SYSREQ026(t *testing.T) {
	data, err := os.ReadFile("Makefile")
	if err != nil {
		t.Fatalf("read Makefile: %v", err)
	}
	makefile := string(data)

	if !regexp.MustCompile(`(?m)^build-fips:`).MatchString(makefile) {
		t.Fatal("Makefile must declare a `build-fips:` target for SYS-REQ-026")
	}

	// Locate the build-fips recipe block (lines until the next blank line or
	// next top-level target) and confirm it enables boringcrypto.
	recipe := buildFipsRecipe(makefile)
	if recipe == "" {
		t.Fatal("build-fips target has no recipe")
	}
	if !strings.Contains(recipe, "GOEXPERIMENT=boringcrypto") {
		t.Fatalf("build-fips recipe must set GOEXPERIMENT=boringcrypto; got:\n%s", recipe)
	}
	if !strings.Contains(recipe, "boringcrypto") || !strings.Contains(recipe, "go build") {
		t.Fatalf("build-fips recipe must build with boringcrypto; got:\n%s", recipe)
	}
}

// buildFipsRecipe extracts the recipe lines that follow the `build-fips:` target
// up to the next blank line or non-indented line.
func buildFipsRecipe(makefile string) string {
	lines := strings.Split(makefile, "\n")
	var out []string
	inTarget := false
	for _, line := range lines {
		if strings.HasPrefix(line, "build-fips:") {
			inTarget = true
			continue
		}
		if inTarget {
			if strings.TrimSpace(line) == "" {
				break
			}
			// recipe lines are tab-indented; a new non-indented line ends the block
			if !strings.HasPrefix(line, "\t") && !strings.HasPrefix(line, " ") {
				break
			}
			out = append(out, line)
		}
	}
	return strings.Join(out, "\n")
}
