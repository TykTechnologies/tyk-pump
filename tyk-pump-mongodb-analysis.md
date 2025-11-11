# Tyk Pump MongoDB Analysis

## Overview

Tyk Pump is an analytics processing system that collects API analytics data from Redis and pumps it to various storage backends. The MongoDB pumps are particularly important as they provide the data storage for Tyk Dashboard analytics.

## MongoDB Pump Types

### 1. Standard Mongo Pump (`mongo`)
- **Purpose**: Stores raw analytics records
- **Collection**: Single collection (default: `tyk_analytics`)
- **Data**: Individual API request/response records
- **Use Case**: Detailed analytics, debugging, raw data analysis

### 2. Mongo Aggregate Pump (`mongo-pump-aggregate`)
- **Purpose**: Stores pre-aggregated analytics data
- **Collections**: 
  - Organization-specific: `z_tyk_analyticz_aggregate_{ORG_ID}`
  - Mixed collection: `tyk_analytics_aggregates` (when `use_mixed_collection: true`)
- **Data**: Aggregated metrics by time periods (hourly/minute)
- **Use Case**: Dashboard analytics, performance monitoring

### 3. Mongo Selective Pump (`mongo-pump-selective`)
- **Purpose**: Stores analytics per organization in separate collections
- **Collections**: `z_tyk_analyticz_{ORG_ID}` (one per organization)
- **Data**: Raw analytics records separated by organization
- **Use Case**: Multi-tenant environments, organization isolation

### 4. Mongo Graph Pump (`mongo-graph`)
- **Purpose**: Stores GraphQL analytics data
- **Data**: GraphQL operation types, fields, errors
- **Use Case**: GraphQL API monitoring

## Configuration Examples

### Basic Mongo Pump Configuration

```json
{
  "pumps": {
    "mongo": {
      "type": "mongo",
      "meta": {
        "collection_name": "tyk_analytics",
        "mongo_url": "mongodb://username:password@hostname:port/db_name",
        "max_insert_batch_size_bytes": 10485760,
        "max_document_size_bytes": 10485760,
        "collection_cap_enable": true,
        "collection_cap_max_size_bytes": 5368709120,
        "driver": "mongo-go",
        "mongo_use_ssl": false
      }
    }
  }
}
```

### Mongo Aggregate Pump Configuration

```json
{
  "pumps": {
    "mongo-pump-aggregate": {
      "type": "mongo-pump-aggregate",
      "meta": {
        "mongo_url": "mongodb://username:password@hostname:port/db_name",
        "use_mixed_collection": true,
        "store_analytics_per_minute": false,
        "aggregation_time": 60,
        "enable_aggregate_self_healing": true,
        "track_all_paths": false,
        "ignore_tag_prefix_list": ["key-", "internal-"],
        "threshold_len_tag_list": 1000,
        "ignore_aggregations": ["tags", "geo"]
      }
    }
  }
}
```

### Environment Variables

```bash
# Standard Mongo Pump
TYK_PMP_PUMPS_MONGO_TYPE=mongo
TYK_PMP_PUMPS_MONGO_META_COLLECTIONNAME=tyk_analytics
TYK_PMP_PUMPS_MONGO_META_MONGOURL=mongodb://localhost:27017/tyk_analytics
TYK_PMP_PUMPS_MONGO_META_MAXINSERTBATCHSIZEBYTES=10485760
TYK_PMP_PUMPS_MONGO_META_MAXDOCUMENTSIZEBYTES=10485760

# Mongo Aggregate Pump
TYK_PMP_PUMPS_MONGOAGG_TYPE=mongo-pump-aggregate
TYK_PMP_PUMPS_MONGOAGG_META_USEMIXEDCOLLECTION=true
TYK_PMP_PUMPS_MONGOAGG_META_STOREANALYTICSPERMINUTE=false
TYK_PMP_PUMPS_MONGOAGG_META_AGGREGATIONTIME=60
TYK_PMP_PUMPS_MONGOAGG_META_ENABLEAGGREGATESELFHEALING=true
```

## Indexes and Database Structure

### Index Creation Behavior

All MongoDB pumps automatically create indexes during initialization unless `omit_index_creation` is set to `true`. The index creation behavior varies by MongoDB type:

- **Standard MongoDB**: Background index creation supported
- **AWS DocumentDB**: No background index creation (causes cursor leaks)
- **Azure CosmosDB**: Limited index support (no TTL indexes)

### Standard Mongo Pump Indexes

```javascript
// Organization index
db.tyk_analytics.createIndex(
  {"orgid": 1}, 
  {background: true}
)

// API index  
db.tyk_analytics.createIndex(
  {"apiid": 1}, 
  {background: true}
)

// Log browser composite index (for analytics dashboard)
db.tyk_analytics.createIndex(
  {"timestamp": -1, "orgid": 1, "apiid": 1, "apikey": 1, "responsecode": 1}, 
  {name: "logBrowserIndex", background: true}
)
```

### Mongo Selective Pump Indexes

```javascript
// API index
db.tyk_analytics_org123.createIndex(
  {"apiid": 1}, 
  {background: true}
)

// TTL index for document expiration (not supported in CosmosDB)
db.tyk_analytics_org123.createIndex(
  {"expireAt": 1}, 
  {expireAfterSeconds: 0, background: true}
)

// Log browser composite index
db.tyk_analytics_org123.createIndex(
  {"timestamp": -1, "apiid": 1, "apikey": 1, "responsecode": 1}, 
  {name: "logBrowserIndex", background: true}
)
```

### Mongo Aggregate Pump Indexes

```javascript
// TTL index for document expiration (not supported in CosmosDB)
db.z_tyk_analyticz_aggregate_org123.createIndex(
  {"expireAt": 1}, 
  {expireAfterSeconds: 0, background: true}
)

// Timestamp index
db.z_tyk_analyticz_aggregate_org123.createIndex(
  {"timestamp": 1}, 
  {background: true}
)

// Organization index
db.z_tyk_analyticz_aggregate_org123.createIndex(
  {"orgid": 1}, 
  {background: true}
)
```

### Mongo Graph Pump Indexes

```javascript
// Uses same indexes as Standard Mongo Pump
db.graph_analytics.createIndex(
  {"orgid": 1}, 
  {background: true}
)

db.graph_analytics.createIndex(
  {"apiid": 1}, 
  {background: true}
)

db.graph_analytics.createIndex(
  {"timestamp": -1, "orgid": 1, "apiid": 1, "apikey": 1, "responsecode": 1}, 
  {name: "logBrowserIndex", background: true}
)
```

### Provider-Specific Index Limitations

#### AWS DocumentDB
```go
// DocumentDB limitations from code:
// - No background index creation (causes cursor leaks)
// - Collection existence check skipped to avoid cursor leaks
if m.dbConf.MongoDBType == StandardMongo {
    exists, errExists := m.collectionExists(collectionName)
    if errExists == nil && exists {
        m.log.Debug("Collection exists, omitting index creation")
        return nil
    }
}
```

#### Azure CosmosDB
```go
// CosmosDB limitations from code:
// - No TTL index support
// - No background index creation
if m.dbConf.MongoDBType != CosmosDB {
    ttlIndex := model.Index{
        Keys:       []model.DBM{{"expireAt": 1}},
        TTL:        0,
        IsTTLIndex: true,
        Background: m.dbConf.MongoDBType == StandardMongo,
    }
    err = m.store.CreateIndex(context.Background(), d, ttlIndex)
}
```

### Index Configuration Options

#### Disable Index Creation
```json
{
  "pumps": {
    "mongo": {
      "meta": {
        "omit_index_creation": true
      }
    }
  }
}
```

#### Environment Variable
```bash
TYK_PMP_PUMPS_MONGO_META_OMITINDEXCREATION=true
```

### Index Performance Considerations

#### Background vs Foreground Creation
- **Standard MongoDB**: Background creation to avoid blocking operations
- **DocumentDB/CosmosDB**: Foreground creation only (no background option)

#### Index Size Impact
```javascript
// Monitor index sizes
db.tyk_analytics.stats().indexSizes

// Check index usage
db.tyk_analytics.aggregate([
  { $indexStats: {} }
])
```

#### Collection Capping
```json
{
  "collection_cap_enable": true,
  "collection_cap_max_size_bytes": 5368709120  // 5GB
}
```

### Index Maintenance

#### Automatic Index Creation
- Indexes are created during pump initialization
- Existing collections skip index creation (Standard MongoDB only)
- Failed index creation is logged but doesn't stop pump operation

#### Manual Index Management
```javascript
// Check existing indexes
db.tyk_analytics.getIndexes()

// Drop specific index
db.tyk_analytics.dropIndex("logBrowserIndex")

// Recreate indexes
db.tyk_analytics.reIndex()
```

#### Index Optimization Queries
```javascript
// Find unused indexes
db.tyk_analytics.aggregate([
  { $indexStats: {} },
  { $match: { "accesses.ops": { $lt: 100 } } }
])

// Monitor index performance
db.tyk_analytics.aggregate([
  { $indexStats: {} },
  { $project: {
      name: 1,
      accessCount: "$accesses.ops",
      scanCount: "$accesses.scan"
    }
  }
])
```

## Upsert Operations

### Mongo Aggregate Pump Upsert Strategy

The aggregate pump uses a sophisticated upsert strategy with two phases:

#### Phase 1: Incremental Updates
```javascript
// Query to find existing document
{
  "orgid": "org123",
  "timestamp": ISODate("2024-01-15T10:00:00Z")
}

// Update document with $inc operations
{
  "$inc": {
    "total.hits": 1,
    "total.success": 1,
    "total.totalrequesttime": 150.5,
    "apiid.api123.hits": 1,
    "apiid.api123.success": 1,
    "errors.404.hits": 1,
    "errors.404.errortotal": 1
  },
  "$set": {
    "timestamp": ISODate("2024-01-15T10:00:00Z"),
    "expireAt": ISODate("2024-02-15T10:00:00Z"),
    "timeid.year": 2024,
    "timeid.month": 1,
    "timeid.day": 15,
    "timeid.hour": 10,
    "lasttime": ISODate("2024-01-15T10:05:30Z")
  },
  "$max": {
    "total.maxlatency": 250,
    "apiid.api123.maxlatency": 250
  },
  "$min": {
    "total.minlatency": 50,
    "apiid.api123.minlatency": 50
  }
}
```

#### Phase 2: Average Calculations
```javascript
// Second upsert to calculate averages and update lists
{
  "$set": {
    "total.requesttime": 150.5,
    "total.latency": 125.0,
    "total.upstreamlatency": 75.0,
    "lists.apiid": [
      {
        "hits": 100,
        "success": 95,
        "error": 5,
        "requesttime": 150.5,
        "identifier": "api123",
        "humanidentifier": "User API"
      }
    ],
    "lists.errors": [
      {
        "hits": 5,
        "error": 5,
        "requesttime": 200.0,
        "identifier": "404",
        "humanidentifier": "404"
      }
    ]
  }
}
```

### Standard Mongo Pump Insert Strategy

```javascript
// Batch insert of raw analytics records
db.tyk_analytics.insertMany([
  {
    "_id": ObjectId("..."),
    "method": "GET",
    "host": "api.example.com",
    "path": "/users/123",
    "responsecode": 200,
    "apikey": "key123",
    "apiid": "api123",
    "orgid": "org123",
    "timestamp": ISODate("2024-01-15T10:05:30Z"),
    "requesttime": 150,
    "rawrequest": "...",
    "rawresponse": "...",
    "latency": {
      "total": 150,
      "upstream": 100
    }
  }
])
```

## Aggregation Logic

### Time-based Aggregation

The aggregate pump groups records by time periods:

```go
// From analytics/aggregate.go
func setAggregateTimestamp(dbIdentifier string, asTime time.Time, aggregationTime int) time.Time {
    // If aggregationTime is 60, group by hour
    if aggregationTime == 60 {
        return time.Date(asTime.Year(), asTime.Month(), asTime.Day(), asTime.Hour(), 0, 0, 0, asTime.Location())
    }
    
    // Otherwise, use custom aggregation time (1-60 minutes)
    return time.Date(asTime.Year(), asTime.Month(), asTime.Day(), asTime.Hour(), asTime.Minute(), 0, 0, asTime.Location())
}
```

### Dimension Aggregation

The system aggregates data across multiple dimensions:

```go
// Dimensions include:
- APIID: Per API aggregation
- Errors: Per HTTP status code
- Versions: Per API version
- APIKeys: Per API key
- OauthIDs: Per OAuth ID
- Geo: Per geographic location
- Tags: Per custom tags
- Endpoints: Per API endpoint
- KeyEndpoint: Per key + endpoint combination
- OauthEndpoint: Per OAuth + endpoint combination
```

## Self-Healing Mechanism

The aggregate pump includes a self-healing feature to handle MongoDB's 16MB document size limit:

```go
func (m *MongoAggregatePump) ShouldSelfHeal(err error) bool {
    const StandardMongoSizeError = "Size must be between 0 and"
    const CosmosSizeError = "Request size is too large"
    const DocDBSizeError = "Resulting document after update is larger than"
    
    if m.dbConf.EnableAggregateSelfHealing {
        if strings.Contains(err.Error(), StandardMongoSizeError) || 
           strings.Contains(err.Error(), CosmosSizeError) || 
           strings.Contains(err.Error(), DocDBSizeError) {
            
            // Reduce aggregation time by half
            m.divideAggregationTime()
            // Reset timestamp to create new document
            analytics.SetlastTimestampAgggregateRecord(m.dbConf.MongoURL, time.Time{})
            return true
        }
    }
    return false
}
```

## Query Examples

### Find Analytics by Organization
```javascript
db.tyk_analytics.find({"orgid": "org123"})
```

### Find Aggregated Data by Time Range
```javascript
db.z_tyk_analyticz_aggregate_org123.find({
  "timestamp": {
    $gte: ISODate("2024-01-15T00:00:00Z"),
    $lt: ISODate("2024-01-16T00:00:00Z")
  }
})
```

### Find Top APIs by Hit Count
```javascript
db.z_tyk_analyticz_aggregate_org123.aggregate([
  { $unwind: "$lists.apiid" },
  { $sort: { "lists.apiid.hits": -1 } },
  { $limit: 10 }
])
```

### Find Error Patterns
```javascript
db.z_tyk_analyticz_aggregate_org123.aggregate([
  { $unwind: "$lists.errors" },
  { $match: { "lists.errors.error": { $gt: 0 } } },
  { $sort: { "lists.errors.error": -1 } }
])
```

## Performance Considerations

### Batch Processing
- Default batch size: 10MB
- Configurable via `max_insert_batch_size_bytes`
- Concurrent processing with goroutines

### Document Size Management
- Default max document size: 10MB
- Automatic truncation of raw request/response data
- Self-healing for aggregate documents

### Index Optimization
- Background index creation
- Composite indexes for common query patterns
- TTL indexes for automatic cleanup

### Collection Capping
```json
{
  "collection_cap_enable": true,
  "collection_cap_max_size_bytes": 5368709120  // 5GB
}
```

## Advanced Configuration Options

### SSL/TLS Configuration
```json
{
  "mongo_use_ssl": true,
  "mongo_ssl_insecure_skip_verify": false,
  "mongo_ssl_allow_invalid_hostnames": false,
  "mongo_ssl_ca_file": "/path/to/ca.pem",
  "mongo_ssl_pem_keyfile": "/path/to/client.pem"
}
```

### Driver Configuration
```json
{
  "driver": "mongo-go",  // or "mgo" for legacy support
  "mongo_session_consistency": "strong",  // strong, monotonic, eventual
  "mongo_direct_connection": false
}
```

### Database Type Support
```json
{
  "mongo_db_type": 0  // 0=StandardMongo, 1=AWSDocumentDB, 2=CosmosDB
}
```

## Monitoring and Troubleshooting

### Common Issues

1. **Document Size Limits**: Use self-healing for aggregate pump
2. **Connection Timeouts**: Adjust `connection_timeout` settings
3. **Index Creation Failures**: Check `omit_index_creation` setting
4. **Batch Size Issues**: Monitor `max_insert_batch_size_bytes`

### Logging
```json
{
  "log_level": "info"  // debug, info, warning, error
}
```

### Health Checks
- Monitor collection sizes
- Check index usage statistics
- Verify TTL index functionality
- Monitor aggregation performance

## Best Practices

1. **Use appropriate aggregation times** based on data volume
2. **Enable self-healing** for production aggregate pumps
3. **Monitor document sizes** to prevent 16MB limit issues
4. **Use collection capping** for raw analytics data
5. **Configure proper indexes** for your query patterns
6. **Separate organizations** using selective pump for multi-tenant setups
7. **Use mixed collections** for cross-organization analytics
8. **Monitor tag counts** to prevent performance issues

This analysis provides a comprehensive understanding of how Tyk Pump's MongoDB integration works, including configuration options, data structures, and operational considerations for MongoDB consultancy sessions. 