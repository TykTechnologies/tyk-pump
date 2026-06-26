# SW-REQ-095: Per-backend filter input isolation

## Intent
`main.filterData` shall build a backend-specific view of a decoded analytics
batch without mutating the shared input batch. One backend's filters, omit
detailed recording, size trimming, ignore-fields list, or raw payload decoding
must not affect the records observed by sibling backends in the same fan-out.

## Motivation
TT-5776 fixed a shared-slice alias bug. The historical implementation used
`filteredKeys := keys[:]` and then wrote filtered/transformed records into that
slice. Because the slice shared the input backing array, filtering for one pump
could reorder or overwrite records in the batch being dispatched to other
pumps. The fix allocates a separate slice before applying per-backend
transforms.

## Formalization
```
when per_backend_transform_configured delivery shall always satisfy shared_dispatch_batch_preserved
```

Variables are declared in `specs/software/variables/delivery.vars.yaml`.

## Code References
- `main.go:filterData` allocates `filteredKeys := make([]interface{},
  len(keys))` and copies `keys` before writing backend-specific transformed
  records into the filtered view.
- `main.go:writeToPumps` launches one `execPumpWriting` goroutine per
  configured pump; each goroutine invokes `filterData` for its own pump.

## Evidence
- `main_test.go:TestFilterData_DoesNotMutateInputBatch` applies a per-backend
  allow-list filter and asserts the filtered output is correct while the
  caller's input slice keeps its original length, order, and record values.
- `main_test.go:TestFilterData_TransformsDoNotMutateInputBatch` applies
  omit-detailed-recording, max-record-size trimming, ignore-fields removal, and
  raw payload decoding, then asserts the backend-specific transformed record is
  returned while the original shared dispatch record is unchanged.
- Focused race evidence:
  `go test -race -count=1 -run 'TestFilterData_DoesNotMutateInputBatch|TestFilterData_TransformsDoNotMutateInputBatch|TestWriteDataWithFilters' .`
