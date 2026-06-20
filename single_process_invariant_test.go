package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// clusteringImportSubstrings are import-path fragments that would indicate
// internal clustering / leader-election / inter-instance coordination
// machinery. SYS-REQ-027 asserts tyk-pump ships none of them: it is a
// single-process daemon and scaling is operator-deployed via independent
// process replication, not built-in coordination.
//
// Redis "cluster" mode (the storage client connecting to a clustered Redis) is
// NOT internal clustering and is deliberately excluded — it is a property of the
// shared backend, not of tyk-pump coordinating across its own instances.
var clusteringImportSubstrings = []string{
	"hashicorp/raft",
	"hashicorp/memberlist",
	"hashicorp/serf",
	"etcd-io/etcd",
	"go.etcd.io/etcd",
	"olric",
	"hashicorp/consul",
	"leaderelection", // k8s client-go leader election
	"election",
}

// sourceImportsClusteringMachinery scans the tyk-pump module's own Go sources
// (excluding test files and vendored/demo code) for any import path that
// matches a clustering/coordination fragment.
func sourceImportsClusteringMachinery(t *testing.T) (string, bool) {
	t.Helper()
	var hit string
	found := false
	_ = filepath.WalkDir(".", func(path string, d os.DirEntry, err error) error {
		if err != nil || found {
			return nil
		}
		if d.IsDir() {
			// Skip non-source dirs.
			base := d.Name()
			if base == ".git" || base == "vendor" || base == ".proof" || base == "specs" {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		data, readErr := os.ReadFile(path)
		if readErr != nil {
			return nil
		}
		content := string(data)
		for _, frag := range clusteringImportSubstrings {
			if importsFragment(content, frag) {
				hit = path + " imports " + frag
				found = true
				return filepath.SkipAll
			}
		}
		return nil
	})
	return hit, found
}

// importsFragment returns true iff frag appears inside a quoted import path in
// the file's import block(s).
func importsFragment(content, frag string) bool {
	for _, line := range strings.Split(content, "\n") {
		l := strings.TrimSpace(line)
		// import lines are quoted paths, optionally with an alias.
		if !strings.Contains(l, "\"") {
			continue
		}
		// crude but adequate: a quoted token containing the fragment.
		if strings.Contains(l, frag) && strings.Count(l, "\"") >= 2 {
			return true
		}
	}
	return false
}

// Verifies: SYS-REQ-027
// SYS-REQ-027:nominal:nominal
//
// MCDC SYS-REQ-027: single_process_only=T => TRUE
//
// SYS-REQ-027 is the architectural invariant "each tyk-pump instance runs as a
// single-process daemon: no internal clustering, no leader election, no
// inter-instance coordination machinery". The single boolean condition
// (single_process_only) is observable in pure Go by scanning the module's own
// sources for any clustering/consensus/leader-election import — there is none,
// so the invariant holds (single_process_only=T) and this test drives the
// satisfied TRUE row.
//
// The FALSE row (single_process_only=F => FALSE) is the negation the invariant
// forbids: a build that pulled in raft/memberlist/leader-election machinery.
// The released module never ships that, so there is no honest runtime witness
// for it — it is a bare environment/architectural invariant whose negated row
// is structurally absent rather than produced by correct code. The detector's
// classification logic is still proven below over a synthetic source string so
// the witness is trustworthy without fabricating a build state the module does
// not have.
//
//mcdc:ignore SYS-REQ-027: single_process_only=F => FALSE — single_process_only=F means the shipped module imports internal clustering/leader-election/coordination machinery (raft, memberlist, serf, etcd, olric, consul, k8s leader-election). The tyk-pump source tree imports none of these (scanned above), and this assertion plus CI fail before release if such an import were introduced, so a shipped build with single_process_only=F cannot exist. The violation is structurally prevented by the committed dependency set rather than produced by correct code. [reviewed: human:leo] [category: defensive]
func TestSingleProcessInvariant_NoClusteringMachinery_SYSREQ027(t *testing.T) {
	if hit, found := sourceImportsClusteringMachinery(t); found {
		t.Fatalf("SYS-REQ-027 requires single-process operation with no clustering machinery, but found: %s", hit)
	}

	// Prove the detector's independent effect (both arms) so single_process_only=T
	// is trustworthy, not merely a scan that never matches anything.
	if !importsFragment("import (\n\t\"github.com/hashicorp/raft\"\n)", "hashicorp/raft") {
		t.Fatal("detector must flag a raft import as clustering machinery (F-arm of the predicate)")
	}
	if importsFragment("import (\n\t\"github.com/sirupsen/logrus\"\n)", "hashicorp/raft") {
		t.Fatal("detector must NOT flag a non-clustering import (T-arm of the predicate)")
	}
}
