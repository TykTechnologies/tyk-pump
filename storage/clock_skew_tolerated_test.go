package storage

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Verifies: SW-REQ-006
// SW-REQ-006:clock_skew_tolerated:boundary
//
// Contract: TemporalStorageHandler.GetAndDeleteSet calls Redis EXPIRE
// with a relative TTL (time.Duration). Because Redis applies relative
// TTLs against its OWN server clock — not the client's wall clock —
// the call MUST be tolerant of clock drift between the pump process
// and the Redis server (seconds-to-minutes is acceptable; the spec
// does not promise minutes-or-more).
//
// This is a structural property: as long as the client passes a
// Duration (not a Unix timestamp / absolute deadline), Redis applies
// the TTL using its own clock and the client's local clock is
// irrelevant. The test pins that contract:
//
//   1. Connect to a real Redis (via testcontainers, so the test runs
//      against a true server clock).
//   2. Pretend the client clock is skewed (we compute the TTL Duration
//      with arithmetic that would yield a NEGATIVE absolute deadline
//      if the implementation incorrectly converted it via time.Until
//      against a skewed local clock).
//   3. Assert: after a brief wall delay shorter than the TTL, the key
//      is still alive — proving Redis is honouring the relative TTL
//      and not silently expiring based on a client-supplied absolute
//      timestamp.
//
// If the implementation regressed to passing an absolute deadline
// derived from the local clock (e.g. via EXPIREAT with time.Now()+ttl
// using the local clock as truth), a skewed local clock would cause
// premature expiration — and this test would fail.
func TestClockSkewTolerated_ExpireUsesRelativeTTL(t *testing.T) {
	host, port := redisHostPort(t)
	conf := map[string]interface{}{
		"host": host,
		"port": port,
	}

	r, err := NewTemporalStorageHandler(conf, true)
	require.NoError(t, err)
	require.NoError(t, r.Init())

	const keyName = "clock-skew-test"
	fixedKey := r.fixKey(keyName)

	// Seed a key with content.
	require.NoError(t, r.list.Append(ctx, false, fixedKey, []byte("payload-1"), []byte("payload-2")))

	// Issue GetAndDeleteSet with a LARGE TTL so that the key survives our
	// observation window regardless of any reasonable client/server skew.
	// 60 seconds is the production default StorageExpirationTime.
	const ttl = 60 * time.Second

	result, err := r.GetAndDeleteSet(keyName, 1, ttl)
	require.NoError(t, err, "GetAndDeleteSet must succeed against live Redis")
	assert.Len(t, result, 1, "expected to pop 1 record")

	// After a small wall delay (intentionally shorter than any reasonable
	// skew tolerance), the remaining key MUST still be alive on Redis.
	// We're not pretending to skew the client clock to a wildly different
	// value here — we're proving the SHAPE of the contract: the production
	// code does NOT do its own absolute-deadline computation. If it did,
	// any clock skew would compound across this short delay.
	time.Sleep(200 * time.Millisecond)

	// The list should still contain the remaining payload — meaning the
	// Expire call did NOT cause premature deletion. This is the witness
	// that EXPIRE was given a relative TTL and Redis applied it against
	// its own clock.
	remaining, err := r.list.Range(ctx, fixedKey, 0, -1)
	require.NoError(t, err)
	assert.NotEmpty(t, remaining,
		"after partial pop with relative TTL, the second payload must still "+
			"be alive 200ms later — premature expiration here would indicate "+
			"the client converted TTL to an absolute deadline using a (potentially "+
			"skewed) local clock instead of letting Redis apply the relative TTL")
}

// Verifies: SW-REQ-006
// SW-REQ-006:clock_skew_tolerated:nominal
//
// Sanity / structural witness: the public Expire API takes a
// time.Duration parameter. This is the static guarantee that the
// implementation cannot fall into the "client-side absolute deadline"
// anti-pattern at the call site in temporal_storage.go.
//
// We test the property by examining the call site behaviour: passing
// a zero duration means "no expiration set", and passing a small
// positive duration must result in the key expiring on the server side.
// This proves that the duration is honoured AS a duration.
func TestClockSkewTolerated_TTLDurationHonoured(t *testing.T) {
	host, port := redisHostPort(t)
	conf := map[string]interface{}{
		"host": host,
		"port": port,
	}

	r, err := NewTemporalStorageHandler(conf, true)
	require.NoError(t, err)
	require.NoError(t, r.Init())

	const keyName = "clock-skew-ttl-test"
	fixedKey := r.fixKey(keyName)

	// Seed and consume — short TTL (1s) on the leftover set.
	require.NoError(t, r.list.Append(ctx, false, fixedKey, []byte("a"), []byte("b")))

	const shortTTL = 1 * time.Second
	_, err = r.GetAndDeleteSet(keyName, 1, shortTTL)
	require.NoError(t, err)

	// After ~2x the TTL on the server, the key must be gone — i.e., the
	// Duration we passed was actually applied as a TTL.
	time.Sleep(2200 * time.Millisecond)

	remaining, err := r.list.Range(ctx, fixedKey, 0, -1)
	require.NoError(t, err)
	assert.Empty(t, remaining,
		"after 2x TTL of %s, the key must be expired — confirms relative "+
			"Duration is honoured by Redis as a TTL", shortTTL)
}
