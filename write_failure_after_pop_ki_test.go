package main

import (
	"errors"
	"testing"
	"time"

	"github.com/TykTechnologies/tyk-pump/pumps"
	"github.com/TykTechnologies/tyk-pump/serializer"
	pumpstorage "github.com/TykTechnologies/tyk-pump/storage"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Verifies: STK-REQ-002
// Verifies: SYS-REQ-022
// Verifies: KI:write-failure-after-pop-loses-records
// Reproduces: write-failure-after-pop-loses-records
func TestWriteFailureAfterPopLosesRecords_KI(t *testing.T) {
	withAcceptanceGlobals(t)

	msgp := serializer.NewAnalyticsSerializer(serializer.MSGP_SERIALIZER)
	record := acceptanceRecord("post-pop-loss", "org-loss", 200)
	encoded := encodeForAcceptance(t, record, msgp)
	store := &acceptanceStorage{
		values: map[string][]interface{}{
			pumpstorage.ANALYTICS_KEYNAME: {encoded},
		},
	}
	AnalyticsStore = store

	failing := &failingMockPump{
		name:     "all-backends-down",
		writeErr: errors.New("simulated sink outage"),
	}
	Pumps = []pumps.Pump{failing}

	popped, err := AnalyticsStore.GetAndDeleteSet(pumpstorage.ANALYTICS_KEYNAME, 10, time.Minute)
	require.NoError(t, err)
	require.Len(t, popped, 1, "test setup must pop one analytics record before dispatch")

	PreprocessAnalyticsValues(popped, msgp, pumpstorage.ANALYTICS_KEYNAME, false, instrument.NewJob("write-loss-ki"), time.Now(), 1)

	failing.mu.Lock()
	invocations := failing.invocations
	failing.mu.Unlock()
	require.Equal(t, 1, invocations, "test must actually exercise the failing pump")

	store.mu.Lock()
	remaining := append([]interface{}(nil), store.values[pumpstorage.ANALYTICS_KEYNAME]...)
	store.mu.Unlock()

	assert.Empty(t, remaining,
		"KI active: after GetAndDeleteSet pops the batch, an all-pump write failure leaves no retained record, requeue, or DLQ entry")
}
