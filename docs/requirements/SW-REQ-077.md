# SW-REQ-077: Elasticsearch write errors surface through Pump.WriteData

## Parent
This requirement decomposes the Pump interface error-return contract
(`INT-REQ-004`) for the Elasticsearch pump family.

## Intent
`ElasticsearchPump.WriteData` shall surface Elasticsearch operator write
failures, invalid record failures, and per-record indexing failures to the
Pump interface caller instead of only logging those failures and returning nil.

## Motivation
The pump host treats a nil `WriteData` return as success. If Elasticsearch
write failures are only logged, the purge loop cannot account for partial
delivery, retry decisions, or operator-visible failure status through the
common Pump interface.

## Code References
- `pumps/elasticsearch.go:ElasticsearchPump.WriteData`
- `pumps/elasticsearch.go:Elasticsearch{3,5,6,7}Operator.processData`

## Known Issue
- `.proof/known-issues/elasticsearch-writedata-errors-swallowed.yaml`

The current product behavior is not fixed in this pass. The requirement is
KI-gated so the active write-error propagation debt stays separate from
`DEFECT-6`, which records the already-fixed Elasticsearch shutdown/resource
lifetime defect.
