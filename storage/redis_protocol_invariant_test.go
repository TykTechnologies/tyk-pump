package storage

import (
	"testing"
)

// Verifies: SYS-REQ-025
// SYS-REQ-025:nominal:nominal
//
// MCDC SYS-REQ-025: redis_protocol_compatible=T => TRUE
//
// SYS-REQ-025 constrains the temporal-store backend to be Redis-protocol
// compatible. We witness the satisfied (TRUE) row by driving the real storage
// handler against a Redis-protocol server (a redis:7-alpine testcontainer) and
// proving a Redis-protocol round-trip succeeds end to end:
//
//   - NewTemporalStorageHandler + Init() open a connection through the
//     TykTechnologies/storage temporal connector, which speaks RESP.
//   - GetName() reporting "redis" confirms the handler selected the Redis
//     protocol driver.
//   - SetKey followed by a kv.Get round-trip proves the backend accepts and
//     answers Redis-protocol commands; a non-Redis-protocol store could not
//     complete this exchange.
//
// Together these observations witness redis_protocol_compatible=T.
//
// The FALSE row (redis_protocol_compatible=F => FALSE) is the violation the
// shipped storage layer never produces: the ingestion path is hard-wired to the
// Redis-protocol driver, so a non-compatible backend cannot be constructed
// through this handler at all. It therefore has no honest runtime witness here
// and is structurally prevented rather than produced by correct code.
//
//mcdc:ignore SYS-REQ-025: redis_protocol_compatible=F => FALSE — the temporal-store ingestion path is hard-wired to the TykTechnologies/storage RESP/Redis-protocol connector (NewTemporalStorageHandler always selects the "redis" driver, asserted above by GetName()=="redis"). No code path constructs a non-Redis-protocol temporal store, so a redis_protocol_compatible=F backend cannot be built through this handler in any correct build. The violation is structurally absent rather than produced by correct code. [reviewed: human:leo] [category: defensive]
func TestStorageInvariant_RedisProtocolCompatible_SYSREQ025(t *testing.T) {
	host, port := redisHostPort(t) // skips when Docker is unavailable

	conf := map[string]interface{}{
		"host": host,
		"port": port,
	}

	r, err := NewTemporalStorageHandler(conf, true)
	if err != nil {
		t.Fatalf("construct temporal storage handler: %v", err)
	}
	if err := r.Init(); err != nil {
		t.Fatalf("Init against Redis-protocol backend failed: %v", err)
	}

	if name := r.GetName(); name != "redis" {
		t.Fatalf("temporal store must use the Redis protocol driver; GetName()=%q", name)
	}

	// A real Redis-protocol round-trip: SET then GET the same key.
	const keyName = "sysreq025_proto_key"
	const session = "sysreq025_proto_value"
	if err := r.SetKey(keyName, session, 60); err != nil {
		t.Fatalf("SetKey over Redis protocol failed: %v", err)
	}
	got, err := r.kv.Get(ctx, r.fixKey(keyName))
	if err != nil {
		t.Fatalf("Get over Redis protocol failed: %v", err)
	}
	if got != session {
		t.Fatalf("Redis-protocol round-trip mismatch: set %q, got %q", session, got)
	}
}
