// Package pumps — drive-to-100% MC/DC tests for the Kafka pump.
//
// The pre-existing kafka_test.go + kafka_mcdc_test.go already cover the
// nominal Init/WriteData/round-trip surface plus the SASL matrix and the
// negative-batch-bytes warn-only path. The remaining decisions exposed by
// `proof mcdc code report --view decisions --file kafka.go --function Init`
// are all log.Fatal arms (lines 103, 142, 156 in pumps/kafka.go) which call
// os.Exit(1) on bad input.
//
// These arms are documented in two known issues:
//   - KI pumps-logfatal-on-config-decode (line 103: mapstructure.Decode)
//   - KI kafka-logfatal-on-init-mech-and-timeout (lines 142, 156: scram +
//     timeout)
//
// We drive each arm twice:
//
//  1. IN-PROCESS, with logger.GetLogger().ExitFunc swapped to a panic so the
//     MC/DC instrumentation records the branch entry. The withFatalIntercept
//     helper (declared in udp_file_pumps_mcdc_100_test.go) handles the
//     panic-recover dance and restores the original ExitFunc on test cleanup.
//
//  2. OUT-OF-PROCESS, via a subprocess that re-runs the test binary with a
//     BE_KAFKA_FATAL=<sentinel> env var. The child runs the lethal Init() and
//     the parent asserts the child exited with code 1. This proves the
//     production contract (lethal log.Fatal) is intact even after the
//     in-process tests intercept it.
//
// Each test carries the triple form (Trigger / Action / Effect) so the fatal-path audit trail stays continuous.
package pumps

import (
	"os"
	"os/exec"
	"testing"
)

// runKafkaFatalChild forks the current test binary, runs only the named test,
// and asserts the child process exited with code 1 (the log.Fatal contract).
// The child branch is entered via the BE_KAFKA_FATAL env var.
func runKafkaFatalChild(t *testing.T, sentinel string) {
	t.Helper()
	cmd := exec.Command(os.Args[0], "-test.run", "^"+t.Name()+"$", "-test.timeout", "30s")
	cmd.Env = append(os.Environ(), "BE_KAFKA_FATAL="+sentinel)
	out, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatalf("child process expected to exit with non-zero, got nil error; output:\n%s", out)
	}
	exitErr, ok := err.(*exec.ExitError)
	if !ok {
		t.Fatalf("expected *exec.ExitError, got %T: %v; output:\n%s", err, err, out)
	}
	if code := exitErr.ExitCode(); code != 1 {
		t.Fatalf("expected child exit code 1 (log.Fatal contract), got %d; output:\n%s", code, out)
	}
}

// kafkaFatalSentinel returns the BE_KAFKA_FATAL value the child process
// should match to enter the lethal-code-path branch. Returns "" if not in
// child mode.
func kafkaFatalSentinel() string {
	return os.Getenv("BE_KAFKA_FATAL")
}

// -----------------------------------------------------------------------------
// kafka.go:103 — log.Fatal on mapstructure.Decode failure
// KI: pumps-logfatal-on-config-decode
// -----------------------------------------------------------------------------

// TestKafkaPump_Init_DecodeFatal_Subprocess drives the
// `k.log.Fatal("Failed to decode configuration: ", err)` arm at kafka.go:103
// in a child process so we can validate the lethal contract end-to-end.
//
// Triple form:
//   - Trigger: child process is invoked with BE_KAFKA_FATAL=decode and a
//     fundamentally-incompatible config (a plain string).
//   - Action: child calls KafkaPump.Init(<string>), forcing mapstructure to
//     return an error.
//   - Effect: log.Fatal -> os.Exit(1); the parent test asserts the child
//     exit code is 1.
//
// Verifies: KI:pumps-logfatal-on-config-decode
// Verifies: SYS-REQ-004
// MCDC SYS-REQ-004: a_backend_failed=T, other_backends_written=F => FALSE
// Reproduces: pumps-logfatal-on-config-decode
func TestKafkaPump_Init_DecodeFatal_Subprocess(t *testing.T) {
	if kafkaFatalSentinel() == "decode" {
		pump := &KafkaPump{}
		_ = pump.Init("not-a-map") // expected to log.Fatal and os.Exit(1)
		return
	}
	runKafkaFatalChild(t, "decode")
}

// TestKafkaPump_Init_DecodeFatal_InProcess drives the same arm in-process so
// MC/DC coverage tooling records the branch. logger.GetLogger().ExitFunc is
// temporarily swapped to a panic by withFatalIntercept.
//
// Verifies: KI:pumps-logfatal-on-config-decode
// Verifies: SYS-REQ-004
// MCDC SYS-REQ-004: a_backend_failed=T, other_backends_written=F => FALSE
// Reproduces: pumps-logfatal-on-config-decode
func TestKafkaPump_Init_DecodeFatal_InProcess(t *testing.T) {
	withFatalIntercept(t, func() {
		pump := &KafkaPump{}
		_ = pump.Init("not-a-map")
	})
}

// -----------------------------------------------------------------------------
// kafka.go:142 — log.Fatal on scram.Mechanism error
// KI: kafka-logfatal-on-init-mech-and-timeout
// -----------------------------------------------------------------------------

// TestKafkaPump_Init_MechFatal_Subprocess drives the lethal contract for the
// SCRAM mechanism-construction failure. scram.Mechanism calls SASLprep on the
// password (RFC-4013) and SASLprep returns an error on any prohibited Unicode
// codepoint. We use U+2028 (LINE SEPARATOR) which is in stringprep TableC1_2.
//
// Triple form:
//   - Trigger: child invoked with BE_KAFKA_FATAL=mech and SCRAM config with a
//     stringprep-prohibited password.
//   - Action: scram.Mechanism's SASLprep step returns a prohibited-character
//     error; mechErr != nil.
//   - Effect: log.Fatal -> os.Exit(1); parent asserts exit code 1.
//
// Verifies: KI:kafka-logfatal-on-init-mech-and-timeout
// Verifies: SYS-REQ-004
// MCDC SYS-REQ-004: a_backend_failed=T, other_backends_written=F => FALSE
// Reproduces: kafka-logfatal-on-init-mech-and-timeout
func TestKafkaPump_Init_MechFatal_Subprocess(t *testing.T) {
	if kafkaFatalSentinel() == "mech" {
		pump := &KafkaPump{}
		_ = pump.Init(map[string]interface{}{
			"broker":         []string{"localhost:9092"},
			"topic":          "t",
			"use_ssl":        true,
			"sasl_mechanism": "scram",
			"sasl_username":  "u",
			"sasl_password":  "bad pwd", // SASLprep-prohibited codepoint
		})
		return
	}
	runKafkaFatalChild(t, "mech")
}

// TestKafkaPump_Init_MechFatal_InProcess drives the same arm in-process with
// logger.GetLogger().ExitFunc swapped so MC/DC records the branch entry.
//
// Verifies: KI:kafka-logfatal-on-init-mech-and-timeout
// Verifies: SYS-REQ-004
// MCDC SYS-REQ-004: a_backend_failed=T, other_backends_written=F => FALSE
// Reproduces: kafka-logfatal-on-init-mech-and-timeout
func TestKafkaPump_Init_MechFatal_InProcess(t *testing.T) {
	withFatalIntercept(t, func() {
		pump := &KafkaPump{}
		_ = pump.Init(map[string]interface{}{
			"broker":         []string{"localhost:9092"},
			"topic":          "t",
			"use_ssl":        true,
			"sasl_mechanism": "scram",
			"sasl_username":  "u",
			"sasl_password":  "bad pwd",
		})
	})
}

// -----------------------------------------------------------------------------
// kafka.go:156 — log.Fatal on strconv.ParseFloat error for timeout
// KI: kafka-logfatal-on-init-mech-and-timeout
// -----------------------------------------------------------------------------

// TestKafkaPump_Init_TimeoutFloatFatal_Subprocess drives the lethal contract
// for the timeout-parse-fallback failure. When timeout is a string that
// time.ParseDuration cannot parse AND strconv.ParseFloat also cannot parse,
// Init log.Fatals.
//
// Triple form:
//   - Trigger: child invoked with BE_KAFKA_FATAL=timeout and a non-numeric,
//     non-duration timeout string ("not-a-time").
//   - Action: time.ParseDuration fails, strconv.ParseFloat also fails;
//     floatErr != nil.
//   - Effect: log.Fatal -> os.Exit(1); parent asserts exit code 1.
//
// Verifies: KI:kafka-logfatal-on-init-mech-and-timeout
// Verifies: SYS-REQ-004
// MCDC SYS-REQ-004: a_backend_failed=T, other_backends_written=F => FALSE
// Reproduces: kafka-logfatal-on-init-mech-and-timeout
func TestKafkaPump_Init_TimeoutFloatFatal_Subprocess(t *testing.T) {
	if kafkaFatalSentinel() == "timeout" {
		pump := &KafkaPump{}
		_ = pump.Init(map[string]interface{}{
			"broker":  []string{"localhost:9092"},
			"topic":   "t",
			"timeout": "not-a-time",
		})
		return
	}
	runKafkaFatalChild(t, "timeout")
}

// TestKafkaPump_Init_TimeoutFloatFatal_InProcess drives the same arm
// in-process with ExitFunc swapped so MC/DC records the branch entry.
//
// Verifies: KI:kafka-logfatal-on-init-mech-and-timeout
// Verifies: SYS-REQ-004
// MCDC SYS-REQ-004: a_backend_failed=T, other_backends_written=F => FALSE
// Reproduces: kafka-logfatal-on-init-mech-and-timeout
func TestKafkaPump_Init_TimeoutFloatFatal_InProcess(t *testing.T) {
	withFatalIntercept(t, func() {
		pump := &KafkaPump{}
		_ = pump.Init(map[string]interface{}{
			"broker":  []string{"localhost:9092"},
			"topic":   "t",
			"timeout": "not-a-time",
		})
	})
}
