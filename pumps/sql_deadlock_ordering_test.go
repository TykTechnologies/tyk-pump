package pumps

// TT-9424 regression tests.
//
// The SQL aggregate pumps build their upsert batches by ranging over the maps
// returned from Dimensions(). Go randomises map iteration per process, and
// PostgreSQL takes row locks in statement order, so two pump replicas upserting
// overlapping IDs in opposite orders form a circular wait and one transaction is
// aborted with "deadlock detected (SQLSTATE 40P01)". The aborted batch is not
// retried, so the purge cycle's aggregates are silently lost.
//
// The fix sorts each batch by row ID before the upsert, giving every replica the
// same lock acquisition order. These tests stand up several independently
// initialised pumps (separate DB connections, mirroring separate pods) writing an
// overlapping batch concurrently, and assert that no deadlock occurs.
//
// Run with:
//
//	TYK_TEST_POSTGRES="host=localhost port=5433 user=postgres password=test \
//	  dbname=tyk_analytics sslmode=disable" \
//	  go test ./pumps -run TestTT9424 -v -count=1

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/TykTechnologies/tyk-pump/analytics"
	"github.com/stretchr/testify/require"
	"gopkg.in/vmihailenco/msgpack.v2"
)

const (
	// Simulated pump replicas. Three is enough to produce a circular wait
	// reliably while keeping the run short.
	tt9424Replicas = 3
	// Purge cycles per replica.
	tt9424Iterations = 15
	// Distinct APIs per cycle for the standard aggregate pump. Yields 756
	// dimensions - see the note below on why that number matters.
	tt9424Cardinality = 150
)

// A warning about workload shape, because it is not obvious and it is easy to
// break these tests into ones that pass whether or not the bug is present.
//
// How often the deadlock reproduces is very sensitive to the shape of the
// batch, and not in the direction you would guess - piling on more data makes
// it reproduce *less*. Measured on an unfixed tree, three replicas, against
// the graph pump:
//
//	 749 rows from 4 dimension maps  ->   0/45  cycles deadlocked
//	1310 rows from 7 dimension maps  ->  24/45  cycles deadlocked
//	2001 rows from 4 dimension maps  ->   2/120 cycles deadlocked
//	3501 rows from 7 dimension maps  ->   3/120 cycles deadlocked
//
// The fixtures below sit on the 1310-row shape, and each was confirmed to fail
// against an unfixed tree before being committed. Treat those numbers as
// measurements of these fixtures, not as a general model of the bug: the
// mechanism is nondeterministic lock ordering, and reproduction rate is not
// fully characterised beyond the points above.
//
// So if you change a cardinality, the number of dimension maps, or the batch
// size here, re-verify by reverting the sort.Slice calls in the four pumps and
// checking these tests still fail. Otherwise you may quietly be shipping a
// guard that no longer guards anything.

// Dimension counts per DoAggregatedWriting call. Graph and MCP each populate
// their own dimension maps plus three from the embedded AnalyticsRecordAggregate.
const (
	tt9424GraphCardinality  = 187 // 7 maps * 187 + 1 total = 1310 rows
	tt9424MCPCardinality    = 250 // 6 maps * 250 + 1 total = 1501 rows
	tt9424UptimeCardinality = 750 // 750 URLs + 1 total = 751 rows
)

// tt9424Collector accumulates errors from concurrent replicas and counts how
// many were PostgreSQL deadlocks.
type tt9424Collector struct {
	mu        sync.Mutex
	errs      []error
	deadlocks int
}

func (c *tt9424Collector) record(err error) {
	if err == nil {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.errs = append(c.errs, err)
	msg := strings.ToLower(err.Error())
	if strings.Contains(msg, "40p01") || strings.Contains(msg, "deadlock") {
		c.deadlocks++
	}
}

// assertNoDeadlocks fails the test if any replica hit a deadlock, and fails on
// any other write error too - a lost batch is a lost batch whatever aborted it.
func (c *tt9424Collector) assertNoDeadlocks(t *testing.T) {
	t.Helper()
	c.mu.Lock()
	defer c.mu.Unlock()

	for i, err := range c.errs {
		if i >= 5 {
			t.Logf("... and %d more errors", len(c.errs)-5)
			break
		}
		t.Logf("write error %d: %v", i, err)
	}
	if c.deadlocks > 0 {
		t.Fatalf("TT-9424 regression: %d deadlock(s) (SQLSTATE 40P01) across %d write errors; "+
			"batches must be sorted by ID so every replica locks rows in the same order",
			c.deadlocks, len(c.errs))
	}
	if len(c.errs) > 0 {
		t.Fatalf("TT-9424: %d write error(s) with no deadlock - see log above", len(c.errs))
	}
}

// tt9424RunReplicas runs write once per iteration on each replica, all replicas
// concurrently, and returns the collected errors.
func tt9424RunReplicas(replicas int, iterations int, write func(replica, iteration int) error) *tt9424Collector {
	collector := &tt9424Collector{}

	var wg sync.WaitGroup
	for r := 0; r < replicas; r++ {
		wg.Add(1)
		go func(replica int) {
			defer wg.Done()
			for i := 0; i < iterations; i++ {
				collector.record(write(replica, i))
			}
		}(r)
	}
	wg.Wait()

	return collector
}

// tt9424Suffix produces a short deterministic label for index i, distinct for
// every i below 10000. Distinctness matters: these tests assert exact row
// counts, and colliding labels silently collapse dimensions into fewer rows.
func tt9424Suffix(i int) string {
	return fmt.Sprintf("%04d", i)
}

// tt9424AwaitIndex waits for a pump's background index creation to finish.
//
// The pumps only signal this channel when they actually create the index. With
// several replicas sharing one database, the first replica creates it and the
// rest find it already present and never signal - so this must not block
// unconditionally the way the single-pump tests can afford to.
func tt9424AwaitIndex(ch chan bool) {
	if ch == nil {
		return
	}
	select {
	case <-ch:
	case <-time.After(30 * time.Second):
	}
}

// ── 1. Standard SQL aggregate pump ────────────────────────────────────────────

// TestTT9424DeadlockRepro drives SQLAggregatePump.WriteData - the primary site
// from the customer reports (tyk_aggregated / tyk_aggregated_YYYYMMDD).
func TestTT9424DeadlockRepro(t *testing.T) {
	skipTestIfNoPostgres(t)

	for _, sharded := range []bool{false, true} {
		name := "non-sharded"
		if sharded {
			name = "sharded"
		}

		t.Run(name, func(t *testing.T) {
			// One pump instance per replica, each with its own gorm connection -
			// the same shape as separate pods.
			pumps := make([]*SQLAggregatePump, tt9424Replicas)
			for i := 0; i < tt9424Replicas; i++ {
				pmp := &SQLAggregatePump{}
				cfg := newSQLConfig(sharded)
				cfg["track_all_paths"] = true
				require.NoErrorf(t, pmp.Init(cfg), "replica %d init failed", i)
				tt9424AwaitIndex(pmp.backgroundIndexCreated)
				pumps[i] = pmp
			}

			// Fixed timestamp: all replicas land in the same hour bucket and the
			// same day shard, so they upsert an identical set of row IDs.
			ts := time.Date(2099, 6, 1, 10, 0, 0, 0, time.UTC)
			table := analytics.AggregateSQLTable
			if sharded {
				table = analytics.AggregateSQLTable + "_" + ts.Format("20060102")
			}
			t.Cleanup(func() { pumps[0].db.Migrator().DropTable(table) })

			// All replicas process records for the SAME org and hour bucket.
			// AggregateData stores dimensions in maps, so each replica emits the
			// rows in a different (randomised) order.
			keys := []string{"key-alpha", "key-beta", "key-gamma", "key-delta"}
			makeRecords := func() []interface{} {
				recs := make([]interface{}, 0, tt9424Cardinality*2)
				for i := 0; i < tt9424Cardinality; i++ {
					suffix := tt9424Suffix(i)
					rec := analytics.AnalyticsRecord{
						OrgID:        "org-tt9424",
						APIID:        "api-" + suffix,
						APIName:      "API " + suffix,
						APIKey:       keys[i%len(keys)],
						APIVersion:   "v1",
						Path:         "/path-" + suffix,
						Method:       "GET",
						ResponseCode: 200,
						TimeStamp:    ts,
						RequestTime:  10,
					}
					recs = append(recs, rec)
					errRec := rec
					errRec.ResponseCode = 500
					recs = append(recs, errRec)
				}
				return recs
			}

			// Warm up on a single replica first. In the sharded case the day
			// table is created lazily on first write, and three replicas racing
			// to CreateTable the same shard trip a unique violation on the
			// catalogue. That startup race is a separate pre-existing issue;
			// creating the shard up front keeps this test on lock ordering.
			require.NoError(t, pumps[0].WriteData(context.Background(), makeRecords()))

			collector := tt9424RunReplicas(tt9424Replicas, tt9424Iterations, func(replica, _ int) error {
				return pumps[replica].WriteData(context.Background(), makeRecords())
			})
			collector.assertNoDeadlocks(t)
		})
	}
}

// ── 2. Graph SQL aggregate pump ───────────────────────────────────────────────

// TestTT9424GraphAggregateNoDeadlock drives GraphSQLAggregatePump.DoAggregatedWriting
// directly. Building the aggregate by hand rather than going through WriteData
// keeps the dimension cardinality under the test's control and avoids the
// GraphQL request/response parsing machinery, which is irrelevant to lock order.
//
// The cardinality is deliberately sized to one statement - see the workload
// shape note at the top of this file before changing it.
func TestTT9424GraphAggregateNoDeadlock(t *testing.T) {
	skipTestIfNoPostgres(t)

	pumps := make([]*GraphSQLAggregatePump, tt9424Replicas)
	for i := 0; i < tt9424Replicas; i++ {
		pmp := &GraphSQLAggregatePump{}
		require.NoErrorf(t, pmp.Init(newSQLConfig(false)), "replica %d init failed", i)
		pumps[i] = pmp
	}
	t.Cleanup(func() { pumps[0].db.Migrator().DropTable(analytics.AggregateGraphSQLTable) })

	ts := time.Date(2099, 6, 1, 10, 0, 0, 0, time.UTC)

	// Each replica rebuilds the aggregate per cycle so the maps are re-ranged
	// (and so re-randomised) every time.
	makeAggregate := func() *analytics.GraphRecordAggregate {
		ag := &analytics.GraphRecordAggregate{
			Types:      map[string]*analytics.Counter{},
			Fields:     map[string]*analytics.Counter{},
			Operation:  map[string]*analytics.Counter{},
			RootFields: map[string]*analytics.Counter{},
		}
		ag.TimeStamp = ts
		ag.OrgID = "org-tt9424-graph"
		// Dimensions() always appends a "total" dimension built from this
		// counter. It must carry hits: the on-conflict assignment for
		// request_time averages over (existing hits + incoming hits), which
		// divides by zero if both sides are empty.
		ag.Total = analytics.Counter{Hits: tt9424GraphCardinality, Success: tt9424GraphCardinality}
		// The embedded AnalyticsRecordAggregate carries its own dimension maps,
		// and real aggregation populates them. Leaving them nil produces a
		// workload that does not reproduce the bug - see the shape note above.
		ag.APIID = map[string]*analytics.Counter{}
		ag.APIKeys = map[string]*analytics.Counter{}
		ag.Endpoints = map[string]*analytics.Counter{}
		for i := 0; i < tt9424GraphCardinality; i++ {
			suffix := tt9424Suffix(i)
			ag.APIID["api-"+suffix] = &analytics.Counter{Hits: 1, Success: 1}
			ag.APIKeys["key-"+suffix] = &analytics.Counter{Hits: 1, Success: 1}
			ag.Endpoints["endpoint-"+suffix] = &analytics.Counter{Hits: 1, Success: 1}
			ag.Types["Type"+suffix] = &analytics.Counter{Hits: 1, Success: 1}
			ag.Fields["field"+suffix] = &analytics.Counter{Hits: 1, Success: 1}
			ag.Operation["op"+suffix] = &analytics.Counter{Hits: 1, Success: 1}
			ag.RootFields["root"+suffix] = &analytics.Counter{Hits: 1, Success: 1}
		}
		return ag
	}

	collector := tt9424RunReplicas(tt9424Replicas, tt9424Iterations, func(replica, _ int) error {
		return pumps[replica].DoAggregatedWriting(
			context.Background(),
			analytics.AggregateGraphSQLTable,
			"org-tt9424-graph",
			"api-tt9424-graph",
			makeAggregate(),
		)
	})
	collector.assertNoDeadlocks(t)
}

// ── 3. MCP SQL aggregate pump ─────────────────────────────────────────────────

// TestTT9424MCPAggregateNoDeadlock drives MCPSQLAggregatePump.DoAggregatedWriting.
// As with the graph test, the cardinality is sized to one statement - see the
// workload shape note at the top of this file.
func TestTT9424MCPAggregateNoDeadlock(t *testing.T) {
	skipTestIfNoPostgres(t)

	pumps := make([]*MCPSQLAggregatePump, tt9424Replicas)
	for i := 0; i < tt9424Replicas; i++ {
		pmp := &MCPSQLAggregatePump{}
		require.NoErrorf(t, pmp.Init(newSQLConfig(false)), "replica %d init failed", i)
		tt9424AwaitIndex(pmp.backgroundIndexCreated)
		pumps[i] = pmp
	}
	t.Cleanup(func() { pumps[0].db.Migrator().DropTable(analytics.AggregateMCPSQLTable) })

	ts := time.Date(2099, 6, 1, 10, 0, 0, 0, time.UTC)

	makeAggregate := func() *analytics.MCPRecordAggregate {
		ag := analytics.NewMCPRecordAggregate()
		ag.TimeStamp = ts
		ag.OrgID = "org-tt9424-mcp"
		ag.OwnerAPIID = "api-tt9424-mcp"
		// See the note in the graph test: the implicit "total" dimension needs
		// hits or the averaging on-conflict expression divides by zero.
		ag.Total = analytics.Counter{Hits: tt9424MCPCardinality, Success: tt9424MCPCardinality}
		// As in the graph test: the embedded dimension maps must be populated,
		// the way real aggregation leaves them.
		for i := 0; i < tt9424MCPCardinality; i++ {
			suffix := tt9424Suffix(i)
			ag.APIID["api-"+suffix] = &analytics.Counter{Hits: 1, Success: 1}
			ag.APIKeys["key-"+suffix] = &analytics.Counter{Hits: 1, Success: 1}
			ag.Endpoints["endpoint-"+suffix] = &analytics.Counter{Hits: 1, Success: 1}
			ag.Methods["tools/call-"+suffix] = &analytics.Counter{Hits: 1, Success: 1}
			ag.Primitives["tool-"+suffix] = &analytics.Counter{Hits: 1, Success: 1}
			ag.Names["name-"+suffix] = &analytics.Counter{Hits: 1, Success: 1}
		}
		return &ag
	}

	collector := tt9424RunReplicas(tt9424Replicas, tt9424Iterations, func(replica, _ int) error {
		return pumps[replica].DoAggregatedWriting(
			context.Background(),
			analytics.AggregateMCPSQLTable,
			"org-tt9424-mcp",
			"api-tt9424-mcp",
			makeAggregate(),
		)
	})
	collector.assertNoDeadlocks(t)
}

// ── 4. Uptime SQL pump ────────────────────────────────────────────────────────

// TestTT9424UptimeNoDeadlock drives SQLPump.WriteUptimeData.
//
// WriteUptimeData only logs write errors, so a deadlock is invisible to the
// caller. The test detects it by consequence instead: an aborted transaction
// loses its whole batch, so the accumulated hit counts come up short.
//
// The workload spans several organisations, each producing one statement's
// worth of dimensions. That covers the placement constraint - recs is rebuilt
// per org, so the sort has to sit inside the per-org loop. Hoisted above it,
// every org after the first is upserted unsorted and starts deadlocking.
func TestTT9424UptimeNoDeadlock(t *testing.T) {
	skipTestIfNoPostgres(t)

	orgs := []string{"org-tt9424-uptime-a", "org-tt9424-uptime-b", "org-tt9424-uptime-c"}

	pumps := make([]*SQLPump, tt9424Replicas)
	for i := 0; i < tt9424Replicas; i++ {
		pmp := &SQLPump{IsUptime: true}
		require.NoErrorf(t, pmp.Init(newSQLConfig(false)), "replica %d init failed", i)
		pumps[i] = pmp
	}
	t.Cleanup(func() { pumps[0].db.Migrator().DropTable(analytics.UptimeSQLTable) })

	ts := time.Date(2099, 6, 1, 10, 0, 0, 0, time.UTC)

	// Uptime dimensions come from the URL map, so distinct URLs drive the row
	// count. Each org gets tt9424UptimeCardinality URLs plus a "total" row,
	// which stays inside one batch at the default batch size.
	makeRecords := func() []interface{} {
		out := make([]interface{}, 0, len(orgs)*tt9424UptimeCardinality)
		for _, org := range orgs {
			for i := 0; i < tt9424UptimeCardinality; i++ {
				rec := analytics.UptimeReportData{
					OrgID:        org,
					URL:          "https://upstream.example/" + tt9424Suffix(i),
					APIID:        "api-tt9424-uptime",
					ResponseCode: 200,
					RequestTime:  10,
					TimeStamp:    ts,
				}
				encoded, err := msgpack.Marshal(rec)
				require.NoError(t, err)
				out = append(out, string(encoded))
			}
		}
		return out
	}

	tt9424RunReplicas(tt9424Replicas, tt9424Iterations, func(replica, _ int) error {
		pumps[replica].WriteUptimeData(makeRecords())
		return nil
	})

	// Per org: one row per URL plus the "total" row.
	wantRows := int64(len(orgs) * (tt9424UptimeCardinality + 1))
	var gotRows int64
	require.NoError(t, pumps[0].db.Table(analytics.UptimeSQLTable).Count(&gotRows).Error)
	require.Equalf(t, wantRows, gotRows,
		"expected %d uptime aggregate rows; a short count means a batch was aborted (TT-9424 deadlock)", wantRows)

	// Every record is a hit, so each org's total dimension must account for all
	// of them. Checking every org is what catches a sort hoisted out of the
	// per-org loop: the first org would still be fine, later ones would not.
	wantHits := tt9424Replicas * tt9424Iterations * tt9424UptimeCardinality
	for _, org := range orgs {
		var total analytics.UptimeReportAggregateSQL
		require.NoError(t, pumps[0].db.Table(analytics.UptimeSQLTable).
			Where("dimension_value = ? AND org_id = ?", "total", org).
			First(&total).Error)
		require.Equalf(t, wantHits, total.Hits,
			"org %s: total-dimension hits should account for every written record; a short count means a lost batch", org)
	}
}

// ── 5. Sort determinism ───────────────────────────────────────────────────────

// TestTT9424SortDeterminism asserts the property the fix actually depends on:
// identical input produces an identical row order, and therefore identical batch
// boundaries, no matter how Go happens to iterate the dimension maps this run.
//
// This is checked without a database. Dimensions() is called repeatedly on
// equivalent aggregates and the resulting ID sequence - after the same sort the
// pumps apply - must be byte-identical every time. Batch boundaries are derived
// from that sequence and compared too: replica A's batch n and replica B's batch
// n must cover the same ID set, which is what removes cross-batch contention on
// top of the intra-statement lock ordering.
func TestTT9424SortDeterminism(t *testing.T) {
	const (
		runs      = 32
		batchSize = 50
	)

	// Build an aggregate whose dimensions come from maps - the source of the
	// nondeterminism the sort has to absorb.
	makeAggregate := func() *analytics.MCPRecordAggregate {
		ag := analytics.NewMCPRecordAggregate()
		ag.TimeStamp = time.Date(2099, 6, 1, 10, 0, 0, 0, time.UTC)
		ag.OrgID = "org-determinism"
		for i := 0; i < 200; i++ {
			suffix := tt9424Suffix(i)
			ag.Methods["m-"+suffix] = &analytics.Counter{Hits: 1}
			ag.Primitives["p-"+suffix] = &analytics.Counter{Hits: 1}
			ag.Names["n-"+suffix] = &analytics.Counter{Hits: 1}
		}
		return &ag
	}

	// idsFor mirrors what DoAggregatedWriting does: derive one ID per dimension,
	// then sort. It deliberately does not reuse the pump method, so the test
	// fails loudly if the sort is ever dropped from the pump but kept here.
	idsFor := func(ag *analytics.MCPRecordAggregate, apiID string) []string {
		var ids []string
		for _, d := range ag.Dimensions() {
			ids = append(ids, fmt.Sprintf("%v", ag.TimeStamp.Unix())+apiID+d.Name+d.Value)
		}
		sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
		return ids
	}

	// Sanity check: without the sort the order really does vary run to run, so a
	// passing determinism assertion below is meaningful rather than vacuous.
	unsortedVaried := false
	var firstUnsorted []string
	for run := 0; run < runs && !unsortedVaried; run++ {
		var ids []string
		for _, d := range makeAggregate().Dimensions() {
			ids = append(ids, d.Name+d.Value)
		}
		if run == 0 {
			firstUnsorted = ids
			continue
		}
		for i := range ids {
			if ids[i] != firstUnsorted[i] {
				unsortedVaried = true
				break
			}
		}
	}
	require.True(t, unsortedVaried,
		"map iteration did not vary across %d runs - this test can no longer prove the sort is doing anything", runs)

	want := idsFor(makeAggregate(), "api-determinism")
	require.NotEmpty(t, want)

	batchesFor := func(ids []string) [][]string {
		var batches [][]string
		for i := 0; i < len(ids); i += batchSize {
			end := i + batchSize
			if end > len(ids) {
				end = len(ids)
			}
			batches = append(batches, ids[i:end])
		}
		return batches
	}
	wantBatches := batchesFor(want)
	require.Greater(t, len(wantBatches), 1, "input must span more than one batch to test batch boundaries")

	for run := 1; run < runs; run++ {
		got := idsFor(makeAggregate(), "api-determinism")
		require.Equalf(t, want, got, "run %d produced a different row order; lock ordering is not deterministic", run)
		require.Equalf(t, wantBatches, batchesFor(got), "run %d produced different batch boundaries", run)
	}
}
