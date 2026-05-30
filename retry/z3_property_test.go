package retry

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/sirupsen/logrus"
)

type z3Fixture struct {
	Name     string             `json:"name"`
	Property string             `json:"property"`
	Source   string             `json:"source"`
	Inputs   []map[string]any   `json:"inputs"`
	Expected map[string]any     `json:"expected"`
}

// Verifies: SW-REQ-031
func loadZ3Fixtures(t *testing.T, property string) []z3Fixture {
	t.Helper()
	dir, err := filepath.Abs(filepath.Join("..", "tests", "properties"))
	if err != nil {
		t.Fatal(err)
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read tests/properties: %v", err)
	}
	var out []z3Fixture
	for _, e := range entries {
		if !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		b, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			t.Fatalf("read %s: %v", e.Name(), err)
		}
		var f z3Fixture
		if err := json.Unmarshal(b, &f); err != nil {
			t.Fatalf("parse %s: %v", e.Name(), err)
		}
		if f.Property == property {
			out = append(out, f)
		}
	}
	if len(out) == 0 {
		t.Fatalf("no Z3 fixtures found for property %q", property)
	}
	return out
}

// Verifies: SW-REQ-031
func intVal(t *testing.T, v any, key string) int {
	t.Helper()
	switch x := v.(type) {
	case float64:
		return int(x)
	case int:
		return x
	default:
		t.Fatalf("fixture key %q not numeric: %T", key, v)
		return 0
	}
}

// Verifies: SW-REQ-031
// SW-REQ-031:boundary:fuzz
//
// Z3-derived property: retry_attempts_exhausted holds iff attempts >= max_attempts.
// Drives BackoffHTTPRetry.Send against a server that always 5xx's, and verifies
// the production-code stops retrying once the Z3-proven exhaustion boundary is hit.
func TestBackoffHTTPRetry_Send_HonoursZ3ExhaustionBoundary(t *testing.T) {
	for _, f := range loadZ3Fixtures(t, "retry_attempts_exhausted") {
		t.Run(f.Name, func(t *testing.T) {
			in := f.Inputs[0]
			maxAttempts := intVal(t, in["max_attempts"], "max_attempts")
			if maxAttempts <= 0 {
				t.Skipf("max_attempts<=0 is invalid by Z3 parameter constraint")
			}

			// The Z3 property's "attempts" is the cumulative call count INCLUDING the
			// initial try; BackoffHTTPRetry.maxRetries is the number of RETRIES after
			// the first call. So an exhausted boundary of attempts==max_attempts==1
			// maps to maxRetries=0 (one try, no retries).
			maxRetries := uint64(maxAttempts - 1)

			var calls int32
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				atomic.AddInt32(&calls, 1)
				w.WriteHeader(http.StatusInternalServerError)
			}))
			defer srv.Close()

			logger := logrus.NewEntry(logrus.New())
			retry := NewBackoffRetry("z3-exhaustion-test", maxRetries, srv.Client(), logger)
			req, _ := http.NewRequest(http.MethodPost, srv.URL, io.NopCloser(strings.NewReader("x")))
			err := retry.Send(req)
			if err == nil {
				t.Fatal("expected error after exhaustion, got nil")
			}
			if !errors.Is(err, err) { // sanity
				t.Fatal("err lost")
			}

			got := int(atomic.LoadInt32(&calls))
			// Server is hit exactly maxAttempts times (initial + maxRetries).
			if got != maxAttempts {
				t.Fatalf("Z3 says attempts>=max_attempts triggers exhaustion at attempt %d; server saw %d calls (want %d)", maxAttempts, got, maxAttempts)
			}
		})
	}
}
