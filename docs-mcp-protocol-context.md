# MCP protocol analytics (TT-18010)

MCP records add `effective_protocol_version`, `declared_protocol_version`, and
`protocol_version_source`. `jsonrpc_error_code` stores the final wire error code
for rejected requests, including when detailed traffic recording is disabled.
Old records decode with empty protocol fields and zero error code.

Protobuf MCPStats fields 1–4 are unchanged. New protocol fields use 5–7, and the
final JSON-RPC error uses 8. BSON/JSON names for existing record fields remain
unchanged; SQL columns and Elasticsearch mappings are additive.

This branch is a prerequisite for the Gateway/Dashboard TT-18010 v2 layers. An
exact commit pin supports review and testing; production release readiness still
requires a released Pump dependency. No release tag is created by this work.

Validation uses `MONGO_DRIVER=mongo-go GOWORK=off go test -count=1 ./analytics ./serializer ./pumps -run 'Test.*(MCP|Serializer|Elastic)'`
against MongoDB 7 on localhost:27017. Mongo tests reset their `test` database.
SQL cases requiring unavailable PostgreSQL/MySQL services must be reported as
skipped, not counted as passing storage integration.
