package otel

import (
	"context"

	"github.com/sirupsen/logrus"

	tykmetric "github.com/TykTechnologies/opentelemetry/metric"
)

// Pump write metrics family (pump_metrics). Every series is labelled with the
// configured pump name and its registry type, never with GetName() strings or
// record content, so cardinality is bounded by the operator's config.
//
//nolint:gosec // G101 false positive: metric names and descriptions, not credentials.
const (
	// PumpInitializedMetricName is a 1/0 gauge per configured pump: whether it
	// initialised. It has no unit, so Prometheus gets no _ratio suffix.
	PumpInitializedMetricName        = "tyk.pump.initialized"
	PumpInitializedMetricDescription = "Whether a configured pump initialised (1) or was skipped at startup (0)"

	// PumpWritesMetricName counts WriteData calls by outcome.
	PumpWritesMetricName        = "tyk.pump.writes"
	PumpWritesMetricUnit        = "{write}"
	PumpWritesMetricDescription = "Pump write calls, by outcome"

	// PumpWriteRecordsMetricName counts the records handed to a pump, by the
	// outcome of the write, plus the records its filters removed.
	PumpWriteRecordsMetricName        = "tyk.pump.write.records"
	PumpWriteRecordsMetricUnit        = "{record}"
	PumpWriteRecordsMetricDescription = "Analytics records handed to a pump by write outcome, or removed by its filters"

	// PurgeRecordsMetricName counts the records the purge loop read from
	// temporal storage, by whether they decoded.
	PurgeRecordsMetricName        = "tyk.pump.purge.records"
	PurgeRecordsMetricUnit        = "{record}"
	PurgeRecordsMetricDescription = "Analytics records read by the purge loop, by decode result"
)

// Label keys of the pump write metrics family.
const (
	AttrPump     = "pump"
	AttrPumpType = "pump_type"
	AttrOutcome  = "outcome"
	AttrResult   = "result"
)

// Outcome values for pump writes.
const (
	OutcomeSuccess  = "success"
	OutcomeError    = "error"
	OutcomeTimeout  = "timeout"
	OutcomeFiltered = "filtered" // records only
)

// Result values for purge records.
const (
	PurgeResultDecoded      = "decoded"
	PurgeResultDecodeFailed = "decode_failed"
)

// UnknownPump is the pump and pump_type value used when the real one is not
// known: a configured type that is not registered, or an empty identity.
const UnknownPump = "unknown"

// PumpIdentity identifies one configured pump on every pump metric. Empty
// fields are recorded as "unknown".
type PumpIdentity struct {
	Name string // configured pump name, lower-cased
	Type string // registry key from pumps/init.go, or "unknown"
}

// orUnknown returns v, or UnknownPump when v is empty.
func orUnknown(v string) string {
	if v == "" {
		return UnknownPump
	}
	return v
}

// PumpInitState is one configured pump and whether it initialised.
type PumpInitState struct {
	PumpIdentity
	Initialized bool
}

// pumpInstruments are the counters of the pump write metrics family. They
// only exist while the family is on; tyk.pump.initialized is registered later,
// by RegisterPumpInitObserver.
type pumpInstruments struct {
	writes       *tykmetric.Counter
	writeRecords *tykmetric.Counter
	purgeRecords *tykmetric.Counter
}

// newPumpInstruments creates the family's counters. A counter that fails to
// be created is logged and stays nil, which makes it a no-op.
func newPumpInstruments(provider MetricsProvider, logger logrus.FieldLogger) *pumpInstruments {
	p := &pumpInstruments{}
	for _, c := range []struct {
		dst                     **tykmetric.Counter
		name, description, unit string
	}{
		{&p.writes, PumpWritesMetricName, PumpWritesMetricDescription, PumpWritesMetricUnit},
		{&p.writeRecords, PumpWriteRecordsMetricName, PumpWriteRecordsMetricDescription, PumpWriteRecordsMetricUnit},
		{&p.purgeRecords, PurgeRecordsMetricName, PurgeRecordsMetricDescription, PurgeRecordsMetricUnit},
	} {
		counter, err := provider.NewCounter(c.name, c.description, c.unit)
		if err != nil {
			logger.WithError(err).Errorf("Creating %s counter; it will be a no-op", c.name)
		}
		*c.dst = counter
	}
	return p
}

// pumpFamily returns the family's instruments, or nil when the family is off
// or the receiver is nil.
func (m *MetricInstruments) pumpFamily() *pumpInstruments {
	if m == nil {
		return nil
	}
	return m.pump
}

// RecordPurgeRecords adds one purge batch's decode results. Zero counts add
// nothing.
func (m *MetricInstruments) RecordPurgeRecords(ctx context.Context, decoded, failed int) {
	p := m.pumpFamily()
	if p == nil {
		return
	}
	if decoded > 0 {
		p.purgeRecords.Add(ctx, int64(decoded), tykmetric.StringAttribute(AttrResult, PurgeResultDecoded))
	}
	if failed > 0 {
		p.purgeRecords.Add(ctx, int64(failed), tykmetric.StringAttribute(AttrResult, PurgeResultDecodeFailed))
	}
}

// RecordPumpWrite records one WriteData call: its outcome, the records handed
// to the pump under that outcome, and the records filterData removed. Zero
// record counts add nothing; the write itself is always counted. An empty
// identity is recorded as unknown.
func (m *MetricInstruments) RecordPumpWrite(ctx context.Context, id PumpIdentity, outcome string, records, filtered int) {
	p := m.pumpFamily()
	if p == nil {
		return
	}
	pump := tykmetric.StringAttribute(AttrPump, orUnknown(id.Name))
	pumpType := tykmetric.StringAttribute(AttrPumpType, orUnknown(id.Type))
	outcomeAttr := tykmetric.StringAttribute(AttrOutcome, outcome)

	p.writes.Add(ctx, 1, pump, pumpType, outcomeAttr)
	if records > 0 {
		p.writeRecords.Add(ctx, int64(records), pump, pumpType, outcomeAttr)
	}
	if filtered > 0 {
		p.writeRecords.Add(ctx, int64(filtered), pump, pumpType, tykmetric.StringAttribute(AttrOutcome, OutcomeFiltered))
	}
}

// RegisterPumpInitObserver registers tyk.pump.initialized over a fixed
// snapshot of every configured pump and whether it initialised. It is called
// once, after the pumps are built, and pumps must not change afterwards. It is
// a no-op when the family is off.
func (m *MetricInstruments) RegisterPumpInitObserver(pumps []PumpInitState) error {
	if m.pumpFamily() == nil {
		return nil
	}
	_, err := m.provider.NewObservableGauge(PumpInitializedMetricName, PumpInitializedMetricDescription, "",
		func(_ context.Context, observe tykmetric.Float64Observer) error {
			for _, s := range pumps {
				v := 0.0
				if s.Initialized {
					v = 1
				}
				observe(v, tykmetric.StringAttribute(AttrPump, s.Name), tykmetric.StringAttribute(AttrPumpType, s.Type))
			}
			return nil
		})
	return err
}
