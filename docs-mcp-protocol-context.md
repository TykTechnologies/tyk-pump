# MCP protocol analytics (TT-18010)

MCP records add `effective_protocol_version`, `declared_protocol_version`, and
`protocol_version_source`. `jsonrpc_error_code` stores the JSON-RPC error code
supplied by the producer, including when detailed traffic recording is disabled.
Old records decode with empty protocol fields and zero error code. Successful
paired request accounting is separate Gateway work; this Pump change does not
create missing producer records.

Protobuf MCPStats fields 1–4 are unchanged. Protocol fields use 5–7, and the
signed error code uses int64 field 8. Encoding a native Go int is lossless;
decoding rejects a value outside native-int bounds without changing the caller's
destination. AnalyticsRecord JSON/BSON names, SQL columns and Elasticsearch
mapping names remain unchanged. This qualifies binary protobuf serialization,
not a separate protobuf-JSON API.

Signed32 values have identical old/new protobuf wire encoding. The earlier
977a5dd candidate's int32 reader cannot preserve wider values; the frozen reader
tests explicitly demonstrate that limitation. Pre-context readers ignore the
unknown new fields. A new reader cannot recover values already truncated by an
old producer. Deploy compatible Pump/readers before relying on wide codes or new
context fields. Pump must merge before dependent Gateway/Dashboard module pins
are updated; candidate executables can be tested with unchanged producer pins.
No release tag is created by this work.

Local validation uses explicit, task-owned databases:

```sh
export MONGO_DRIVER=mongo-go
export TYK_TEST_MCP_MONGO_URL=mongodb://127.0.0.1:27018/mcp_compatibility_tests
export TYK_TEST_POSTGRES='host=127.0.0.1 port=55432 user=mcp_ga dbname=mcp_compatibility_tests sslmode=disable'
GOWORK=off go test -count=1 ./analytics ./serializer
GOWORK=off go test -race -count=1 ./analytics ./serializer
GOWORK=off go test -count=1 ./pumps -run 'Test.*(MCP|Serializer|Elastic)'
GOWORK=off go test -race -count=1 ./pumps -run 'Test.*(MCP|Serializer|Elastic)'
```

Provision the selected databases first. MCP tests use unique Mongo collections
and SQL tables with scoped cleanup; they do not drop the Mongo database. Missing
required storage setup or selected skips are not passing qualification. Real
Mongo/PostgreSQL fresh-table and legacy-table migration tests complement SQLite
batch/shard tests and Elasticsearch mapping tests. Mapping tests do not claim a
running Elasticsearch destination. Unrelated destination-driver and boringcrypto
matrices are outside this ticket's selected local validation.

Generated code uses protoc 3.21.12 and protoc-gen-go 1.28.1 followed by the
repository's goimports v0.33.0 step. Generate into a temporary directory and
compare the complete result with `analytics/proto/analytics.pb.go`.
