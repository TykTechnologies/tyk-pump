# SW-REQ-092: MongoSelective document-size arithmetic

## Intent
MongoSelective shall enforce `MaxDocumentSizeBytes` using the exact local
estimate:

```
len(RawRequest) + len(RawResponse) + 1024
```

`RawRequest` and `RawResponse` are each counted once, and the record is skipped
only when that estimate is greater than `MaxDocumentSizeBytes`.

## Motivation
Before TT-6550, MongoSelective counted `RawRequest` twice and omitted
`RawResponse`. Large responses could bypass the local guard and fail during the
backend insert, while some valid request-heavy records could be skipped because
the request payload was double-counted.

## Formalization
```
when selective_document_size_estimated pumps_mongo_selective shall always satisfy raw_request_and_response_counted_once
```

Variables are declared in
`specs/software/variables/pumps-mongo-selective.vars.yaml`.

## Code References
- `pumps/mongo_selective.go:getItemSizeBytes` computes
  `len(RawRequest)+len(RawResponse)+1024` and directly implements this
  requirement.
- `pumps/mongo_selective.go:accumulate` skips records when the computed size is
  negative, which `getItemSizeBytes` uses for oversized documents.
- `pumps/mongo_selective.go:AccumulateSet` applies the size calculation before
  batching.

## Evidence
- `pumps/mongo_selective_test.go:TestMongoSelectivePump_GetItemSizeBytes_CountsRawRequestAndResponseOnce`
  covers exact-threshold retention, RawResponse-driven overflow, and the old
  RawRequest-double-count regression.
- `pumps/mongo_selective_test.go:TestMongoSelectivePump_AccumulateSet` covers
  AccumulateSet-level skip behavior for oversized records.
