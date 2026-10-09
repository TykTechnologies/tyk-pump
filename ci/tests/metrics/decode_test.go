package metrics

import "testing"

// TestPurgeDecodeFailures runs under the "decode" profile, only the base
// unfiltered dummy pump. Values that are not analytics records, pushed onto
// the list the Gateway writes to, count as decode_failed next to the
// Gateway's decoded records, and still reach the pump as empty slots.
func TestPurgeDecodeFailures(t *testing.T) {
	requireProfile(t, "decode")
	waitForPump(t)
	waitForGateway(t)

	const garbage, traffic = 3, 10
	pushToAnalytics(t, "not-an-analytics-record-1", "not-an-analytics-record-2", "not-an-analytics-record-3")
	sendTraffic(t, api1Path, "decode", traffic)

	waitForPoints(t, purgeRecordsFamily, map[string]float64{"decoded": traffic, "decode_failed": garbage}, "result")

	snap := waitForReconciled(t, garbage+traffic, []string{"dummy"})
	expectValue(t, pumpWriteRecordsFamily, snap.records, "dummy/success", garbage+traffic)
}
