package main

import (
	"os"
	"regexp"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

// Verifies: SYS-REQ-026
// SYS-REQ-026:nominal:nominal
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
//mcdc:ignore SYS-REQ-026: fips_build_available=F => FALSE — the requirement is discharged by inspecting the named build artifact (the `build-fips` Makefile target wired to GOEXPERIMENT=boringcrypto). That target is committed in the repository, and this assertion plus CI fail before release if it is removed or de-wired, so a shipped tree with fips_build_available=F cannot exist. The violation is structurally prevented by the committed build system rather than produced by correct code. [reviewed: human:leo] [category: defensive]
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

// Verifies: SYS-REQ-036
// SYS-REQ-036:security_mode_artifact_consistent:nominal
//
// MCDC SYS-REQ-036: fips_release_artifact_variant_preserved=T => TRUE
//
//mcdc:ignore SYS-REQ-036: fips_release_artifact_variant_preserved=F => FALSE — the violation row is a release-configuration regression, not a runtime branch. This static witness parses the committed GoReleaser/workflow YAML and fails before release if a FIPS-labelled package/image is wired to a standard build/package/base image. DEFECT-48 records the historical 206c1d0 regression shape. [reviewed: human:buger] [category: defensive]
func TestReleaseInvariant_FIPSArtifactsUseFIPSBuilds_SYSREQ036(t *testing.T) {
	release := readYAMLFile[goreleaserConfig](t, "ci/goreleaser/goreleaser.yml")

	fipsBuilds := map[string]bool{}
	for _, build := range release.Builds {
		if strings.Contains(strings.ToLower(build.ID), "fips") {
			if !buildHasFIPSSignal(build) {
				t.Fatalf("FIPS build %q must carry a FIPS build signal; env=%v flags=%v", build.ID, build.Env, build.Flags)
			}
			fipsBuilds[build.ID] = true
		}
	}
	if len(fipsBuilds) == 0 {
		t.Fatal("release config must define at least one FIPS build ID")
	}

	foundFIPSPackage := false
	for _, nfpm := range release.NFPMS {
		if nfpm.ID != "fips" && nfpm.PackageName != "tyk-pump-fips" {
			continue
		}
		foundFIPSPackage = true
		if nfpm.PackageName != "tyk-pump-fips" {
			t.Fatalf("FIPS package entry %q must publish package_name tyk-pump-fips, got %q", nfpm.ID, nfpm.PackageName)
		}
		refs := nfpmBuildRefs(nfpm)
		if len(refs) == 0 {
			t.Fatalf("FIPS package entry %q must reference FIPS build IDs", nfpm.ID)
		}
		for _, ref := range refs {
			if !fipsBuilds[ref] {
				t.Fatalf("FIPS package %q references non-FIPS build ID %q; fips builds=%v", nfpm.ID, ref, keys(fipsBuilds))
			}
		}
	}
	if !foundFIPSPackage {
		t.Fatal("release config must publish a tyk-pump-fips package entry")
	}

	workflow := readYAMLFile[githubWorkflow](t, ".github/workflows/release.yml")
	fipsDockerSteps := 0
	for _, job := range workflow.Jobs {
		for _, step := range job.Steps {
			name := strings.ToLower(step.Name)
			if !strings.Contains(name, "fips image") || !strings.Contains(step.Uses, "docker/build-push-action") {
				continue
			}
			fipsDockerSteps++
			buildArgs := step.With["build-args"]
			if !strings.Contains(buildArgs, "BUILD_PACKAGE_NAME=tyk-pump-fips") {
				t.Fatalf("FIPS Docker step %q must install tyk-pump-fips; build args:\n%s", step.Name, buildArgs)
			}
			if strings.Contains(buildArgs, "BUILD_PACKAGE_NAME=tyk-pump\n") {
				t.Fatalf("FIPS Docker step %q must not install the standard package; build args:\n%s", step.Name, buildArgs)
			}
			if !strings.Contains(strings.ToLower(buildArgs), "base_image=") || !strings.Contains(strings.ToLower(buildArgs), "fips") {
				t.Fatalf("FIPS Docker step %q must use a FIPS base image; build args:\n%s", step.Name, buildArgs)
			}
		}
	}
	if fipsDockerSteps == 0 {
		t.Fatal("release workflow must contain at least one FIPS Docker build-push step")
	}
}

type goreleaserConfig struct {
	Builds []goreleaserBuild `yaml:"builds"`
	NFPMS  []goreleaserNFPM  `yaml:"nfpms"`
}

type goreleaserBuild struct {
	ID    string   `yaml:"id"`
	Flags []string `yaml:"flags"`
	Env   []string `yaml:"env"`
}

type goreleaserNFPM struct {
	ID          string   `yaml:"id"`
	PackageName string   `yaml:"package_name"`
	IDs         []string `yaml:"ids"`
	Builds      []string `yaml:"builds"`
}

type githubWorkflow struct {
	Jobs map[string]githubWorkflowJob `yaml:"jobs"`
}

type githubWorkflowJob struct {
	Steps []githubWorkflowStep `yaml:"steps"`
}

type githubWorkflowStep struct {
	Name string            `yaml:"name"`
	Uses string            `yaml:"uses"`
	With map[string]string `yaml:"with"`
}

func readYAMLFile[T any](t *testing.T, path string) T {
	t.Helper()

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}

	var out T
	if err := yaml.Unmarshal(data, &out); err != nil {
		t.Fatalf("parse %s: %v", path, err)
	}
	return out
}

func buildHasFIPSSignal(build goreleaserBuild) bool {
	fields := append(append([]string{}, build.Env...), build.Flags...)
	for _, field := range fields {
		lower := strings.ToLower(field)
		if strings.Contains(lower, "gofips") || strings.Contains(lower, "boringcrypto") || strings.Contains(lower, "tags=fips") {
			return true
		}
	}
	return false
}

func nfpmBuildRefs(nfpm goreleaserNFPM) []string {
	if len(nfpm.IDs) > 0 {
		return nfpm.IDs
	}
	return nfpm.Builds
}

func keys(values map[string]bool) []string {
	out := make([]string, 0, len(values))
	for key := range values {
		out = append(out, key)
	}
	return out
}
