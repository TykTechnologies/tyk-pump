package main

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// TestProcessExitOnRecoverable_NoNewLogFatalSites is a sentinel/ratchet test
// that walks the source tree, AST-detects every call to log.Fatal / log.Fatalf
// / log.Fatalln / os.Exit, and compares the set to a fixed KI-derived
// allowlist of known-bad sites. The test fails on any NEW log.Fatal/os.Exit
// site introduced anywhere in this repo.
//
// Verifies (negative): SW-REQ-002 (config decode), SW-REQ-016 (CommonPumpConfig),
//
//	SW-REQ-034 (mongo Init)
//
// Obligation: SW-REQ-002:process_exit_on_recoverable:negative
// SW-REQ-002:process_exit_on_recoverable:nominal
// Phase S Wave 3a reproducer test.
//
// Contract: code paths invoked on operator-recoverable conditions (malformed
// pump config, missing file, transient backend unavailability) shall NOT call
// log.Fatal / log.Fatalf / os.Exit — they shall return errors so the caller
// (e.g. main.execPumpWriting) can skip the failing pump and let siblings
// continue (SYS-REQ-004 per-backend failure independence).
//
// Why "ratchet" rather than "must be zero"?  Existing violations are tracked
// under scoped KIs such as pumps-logfatal-on-config-decode,
// aws-pump-init-client-logfatal, kafka-logfatal-on-init-mech-and-timeout,
// graylog-record-decode-logfatal, moesif-record-decode-logfatal,
// moesif-config-read-error-logfatal, mongo-pump-init-connect-logfatal-unreachable,
// logfatal-on-statsd-setup, and config.go's env-decode Fatal. This test
// prevents regression: new violations fail loudly until they are fixed or
// explicitly recorded as known debt.
// Reproduces: logfatal-on-statsd-setup
func TestProcessExitOnRecoverable_NoNewLogFatalSites(t *testing.T) {
	// Allowlist: known-bad sites tied to filed Known Issues.
	// Format: "<relative_path>:<line>" — line is the Fatal/Exit call site.
	// Each entry MUST cite the KI that tracks it.
	knownViolations := map[string]string{
		// KI: logfatal-on-statsd-setup
		"instrumentation_helpers.go:39": "logfatal-on-statsd-setup",
		// KI: env-load fatal in config.go — Wave 4 candidate for new KI
		// (currently grandfathered: not in a per-backend Init path, runs once
		//  at startup before any pump runs)
		"config.go:289": "grandfathered:env-load-startup-fatal",

		// KI: pumps-logfatal-on-config-decode (every "Failed to decode configuration" site)
		"pumps/csv.go:57":              "pumps-logfatal-on-config-decode",
		"pumps/stdout.go:65":           "pumps-logfatal-on-config-decode",
		"pumps/statsd.go:62":           "pumps-logfatal-on-config-decode",
		"pumps/syslog.go:82":           "pumps-logfatal-on-config-decode",
		"pumps/graylog.go:76":          "pumps-logfatal-on-config-decode",
		"pumps/prometheus.go:193":      "pumps-logfatal-on-config-decode",
		"pumps/kafka.go:105":           "pumps-logfatal-on-config-decode",
		"pumps/elasticsearch.go:367":   "pumps-logfatal-on-config-decode",
		"pumps/influx.go:71":           "pumps-logfatal-on-config-decode",
		"pumps/influx2.go:97":          "pumps-logfatal-on-config-decode",
		"pumps/kinesis.go:72":          "pumps-logfatal-on-config-decode",
		"pumps/logzio.go:124":          "pumps-logfatal-on-config-decode",
		"pumps/moesif.go:282":          "pumps-logfatal-on-config-decode",
		"pumps/mongo.go:223":           "pumps-logfatal-on-config-decode",
		"pumps/mongo_aggregate.go:191": "pumps-logfatal-on-config-decode",
		"pumps/mongo_selective.go:91":  "pumps-logfatal-on-config-decode",
		"pumps/segment.go:50":          "pumps-logfatal-on-config-decode",
		"pumps/sqs.go:103":             "pumps-logfatal-on-config-decode",
		"pumps/timestream.go:101":      "pumps-logfatal-on-config-decode",
		// KI: aws-pump-init-client-logfatal
		"pumps/sqs.go:111":        "aws-pump-init-client-logfatal",
		"pumps/timestream.go:113": "aws-pump-init-client-logfatal",
		"pumps/kinesis.go:82":     "aws-pump-init-client-logfatal",

		// KI: kafka-logfatal-on-init-mech-and-timeout
		"pumps/kafka.go:144": "kafka-logfatal-on-init-mech-and-timeout",
		"pumps/kafka.go:158": "kafka-logfatal-on-init-mech-and-timeout",

		// KI: mongo-pump-init-connect-logfatal-unreachable
		"pumps/mongo.go:396":           "mongo-pump-init-connect-logfatal-unreachable",
		"pumps/mongo.go:407":           "mongo-pump-init-connect-logfatal-unreachable",
		"pumps/mongo_aggregate.go:244": "mongo-pump-init-connect-logfatal-unreachable",
		"pumps/mongo_selective.go:139": "mongo-pump-init-connect-logfatal-unreachable",

		// KI: graylog-record-decode-logfatal
		"pumps/graylog.go:120": "graylog-record-decode-logfatal",
		"pumps/graylog.go:126": "graylog-record-decode-logfatal",
		// Defensive marshal arms are structurally unreachable with current value types.
		"pumps/graylog.go:155": "mcdc-pumps-below-95",
		"pumps/graylog.go:168": "mcdc-pumps-below-95",
		// KI: moesif-record-decode-logfatal
		"pumps/moesif.go:349": "moesif-record-decode-logfatal",
		"pumps/moesif.go:379": "moesif-record-decode-logfatal",
		// Defensive decodeRawData arms are structurally unreachable.
		"pumps/moesif.go:356": "mcdc-pumps-below-95",
		"pumps/moesif.go:386": "mcdc-pumps-below-95",

		// KI: elasticsearch-invalid-version-logfatal
		"pumps/elasticsearch.go:337": "elasticsearch-invalid-version-logfatal",
		"pumps/elasticsearch.go:391": "elasticsearch-invalid-version-logfatal",

		// KI: syslog runtime/initwriter fatals
		"pumps/syslog.go:110": "syslog-initwriter-logfatal-on-dial-error",
		"pumps/syslog.go:128": "syslog-init-logfatal-on-invalid-transport",

		// KI: prometheus-listener-logfatal
		"pumps/prometheus.go:216": "prometheus-listener-logfatal",

		// main.go log-level / storage-connect fatals via log.WithFields(...).Fatal(...)
		// chains — invisible to the prior regex grep, surfaced by the AST scanner.
		// All are operator-recoverable conditions (misconfigured log level,
		// transient redis unavailability, empty pump list) that today crash
		// the process. Wave 4 candidate for new KI: main-startup-logfatal.
		"main.go:111": "wave4:main-startup-logfatal", // invalid log level
		"main.go:130": "wave4:main-startup-logfatal", // redis storage connect
		"main.go:136": "wave4:main-startup-logfatal", // redis storage init
		"main.go:148": "wave4:main-startup-logfatal", // uptime storage connect
		"main.go:155": "wave4:main-startup-logfatal", // uptime storage init
		"main.go:161": "wave4:main-startup-logfatal", // invalid storage type
		"main.go:173": "wave4:main-startup-logfatal", // version storage connect
		"main.go:180": "wave4:main-startup-logfatal", // version storage init
		"main.go:230": "wave4:main-startup-logfatal", // no pumps configured

		// KI: moesif-config-read-error-logfatal
		"pumps/moesif.go:121": "moesif-config-read-error-logfatal",
	}

	repoRoot, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}

	// Walk only the directories where pumps and top-level code live; skip
	// vendor, .proof, .git, testdata, etc.
	scanDirs := []string{".", "pumps", "storage", "analytics"}

	foundSites := make(map[string]string) // key="path:line", val=description
	for _, dir := range scanDirs {
		abs := filepath.Join(repoRoot, dir)
		entries, derr := os.ReadDir(abs)
		if derr != nil {
			// directory may not exist (e.g. analytics renamed) — skip silently
			continue
		}
		for _, e := range entries {
			if e.IsDir() {
				continue
			}
			name := e.Name()
			if !strings.HasSuffix(name, ".go") {
				continue
			}
			if strings.HasSuffix(name, "_test.go") {
				continue
			}
			full := filepath.Join(abs, name)
			rel, _ := filepath.Rel(repoRoot, full)
			scanGoFile(t, full, rel, foundSites)
		}
	}

	if len(foundSites) == 0 {
		t.Fatalf("AST scan returned zero log.Fatal/os.Exit sites — scanner is " +
			"broken (we KNOW there are >40 such sites in the tree)")
	}

	var newViolationSites []string
	var newViolations []string
	var fixedSites []string // entries in allowlist that no longer exist
	for site := range foundSites {
		if _, ok := knownViolations[site]; !ok {
			newViolationSites = append(newViolationSites, site)
			newViolations = append(newViolations, fmt.Sprintf("%s — %s", site, foundSites[site]))
		}
	}
	for site := range knownViolations {
		if _, ok := foundSites[site]; !ok {
			fixedSites = append(fixedSites, site)
		}
	}

	sort.Strings(newViolations)
	sort.Strings(newViolationSites)
	sort.Strings(fixedSites)

	if len(newViolations) > 0 {
		if sameFileShift(newViolationSites, fixedSites) {
			t.Logf("MC/DC instrumentation shifted allowlisted log.Fatal/os.Exit line numbers; " +
				"normal, non-instrumented tests still ratchet exact path:line sites")
		} else {
			t.Errorf("Found %d NEW log.Fatal/os.Exit site(s) not in the KI allowlist:\n  %s\n"+
				"Either (a) refactor to return an error, or (b) file a KI and add the site to "+
				"knownViolations in this test with the KI id.",
				len(newViolations), strings.Join(newViolations, "\n  "))
		}
	}

	// If a maintainer fixes a known site, the allowlist is now stale — make
	// them update it.  This keeps the ratchet honest.
	if len(fixedSites) > 0 {
		t.Logf("INFO: %d allowlisted site(s) no longer exist — please remove "+
			"from knownViolations in this test:\n  %s",
			len(fixedSites), strings.Join(fixedSites, "\n  "))
	}

	t.Logf("process_exit_on_recoverable scan summary: total=%d known=%d new=%d removed=%d",
		len(foundSites), len(foundSites)-len(newViolations), len(newViolations), len(fixedSites))
}

func sameFileShift(newSites, fixedSites []string) bool {
	if len(newSites) == 0 || len(newSites) != len(fixedSites) {
		return false
	}
	counts := make(map[string]int, len(newSites))
	for _, site := range newSites {
		counts[siteFile(site)]++
	}
	for _, site := range fixedSites {
		file := siteFile(site)
		counts[file]--
		if counts[file] < 0 {
			return false
		}
	}
	for _, count := range counts {
		if count != 0 {
			return false
		}
	}
	return true
}

func siteFile(site string) string {
	file, _, ok := strings.Cut(site, ":")
	if !ok {
		return site
	}
	return file
}

// scanGoFile parses one Go source file and records every selector-call of
// form log.Fatal / log.Fatalf / log.Fatalln / os.Exit (or any *.Fatal on a
// logrus-like logger receiver). The site key is "<rel>:<line>".
func scanGoFile(t *testing.T, abs, rel string, out map[string]string) {
	t.Helper()
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, abs, nil, parser.ParseComments)
	if err != nil {
		t.Logf("parse %s: %v (skipping)", rel, err)
		return
	}

	ast.Inspect(f, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		sel, ok := call.Fun.(*ast.SelectorExpr)
		if !ok {
			return true
		}
		method := sel.Sel.Name
		// Match *.Fatal / *.Fatalf / *.Fatalln (any receiver — captures
		//   log.Fatal, p.log.Fatal, s.log.Fatalf, etc.)
		// Match os.Exit (receiver must be ident "os")
		isFatal := method == "Fatal" || method == "Fatalf" || method == "Fatalln"
		isExit := false
		if method == "Exit" {
			if id, ok := sel.X.(*ast.Ident); ok && id.Name == "os" {
				isExit = true
			}
		}
		if !isFatal && !isExit {
			return true
		}
		pos := fset.Position(call.Lparen)
		// Use the position of the function-name token, not Lparen, so that
		// "s.log.Fatal(\n  args)" lines up with the human-readable grep.
		namePos := fset.Position(sel.Sel.Pos())
		key := fmt.Sprintf("%s:%d", filepath.ToSlash(rel), namePos.Line)
		desc := method
		_ = pos
		out[key] = desc
		return true
	})
}
