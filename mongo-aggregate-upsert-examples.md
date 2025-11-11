# MongoDB Aggregate Pump Upsert Query Examples

Based on the actual Tyk Pump code implementation, here are the specific MongoDB upsert queries that the Mongo Aggregate pump generates.

## Overview

The Mongo Aggregate pump performs **two-phase upserts** for each aggregation period:

1. **Phase 1**: Incremental updates using `$inc`, `$set`, `$max`, and `$min` operators
2. **Phase 2**: Average calculations and list updates using `$set` operators

## Collection Structure

### Collection Names
- **Organization-specific**: `z_tyk_analyticz_aggregate_{ORG_ID}`
- **Mixed collection**: `tyk_analytics_aggregates` (when `use_mixed_collection: true`)

### Query Filter
```javascript
{
  "orgid": "org123",
  "timestamp": ISODate("2024-01-15T10:00:00Z")
}
```

## Phase 1: Incremental Updates (AsChange)

### Basic Structure
```javascript
db.z_tyk_analyticz_aggregate_org123.updateOne(
  {
    "orgid": "org123",
    "timestamp": ISODate("2024-01-15T10:00:00Z")
  },
  {
    "$inc": {
      // Incremental counters
    },
    "$set": {
      // Set values
    },
    "$max": {
      // Maximum values
    },
    "$min": {
      // Minimum values (only for successful requests)
    }
  },
  { upsert: true }
)
```

### Complete Phase 1 Example

```javascript
db.z_tyk_analyticz_aggregate_org123.updateOne(
  {
    "orgid": "org123",
    "timestamp": ISODate("2024-01-15T10:00:00Z")
  },
  {
    "$inc": {
      // Total counters
      "total.hits": 1,
      "total.success": 1,
      "total.errortotal": 0,
      "total.totalrequesttime": 150.5,
      "total.totalupstreamlatency": 100,
      "total.totallatency": 150,
      
      // API-specific counters
      "apiid.api123.hits": 1,
      "apiid.api123.success": 1,
      "apiid.api123.errortotal": 0,
      "apiid.api123.totalrequesttime": 150.5,
      "apiid.api123.totalupstreamlatency": 100,
      "apiid.api123.totallatency": 150,
      
      // Error counters
      "errors.404.hits": 0,
      "errors.404.errortotal": 0,
      "errors.500.hits": 0,
      "errors.500.errortotal": 0,
      
      // API Key counters
      "apikeys.key123.hits": 1,
      "apikeys.key123.success": 1,
      "apikeys.key123.errortotal": 0,
      "apikeys.key123.totalrequesttime": 150.5,
      "apikeys.key123.totalupstreamlatency": 100,
      "apikeys.key123.totallatency": 150,
      
      // Endpoint counters
      "endpoints./users/123.hits": 1,
      "endpoints./users/123.success": 1,
      "endpoints./users/123.errortotal": 0,
      "endpoints./users/123.totalrequesttime": 150.5,
      "endpoints./users/123.totalupstreamlatency": 100,
      "endpoints./users/123.totallatency": 150,
      
      // Key-Endpoint combination
      "keyendpoints.key123./users/123.hits": 1,
      "keyendpoints.key123./users/123.success": 1,
      "keyendpoints.key123./users/123.errortotal": 0,
      "keyendpoints.key123./users/123.totalrequesttime": 150.5,
      "keyendpoints.key123./users/123.totalupstreamlatency": 100,
      "keyendpoints.key123./users/123.totallatency": 150
    },
    "$set": {
      // Timestamp and metadata
      "timestamp": ISODate("2024-01-15T10:00:00Z"),
      "expireAt": ISODate("2024-02-15T10:00:00Z"),
      "timeid.year": 2024,
      "timeid.month": 1,
      "timeid.day": 15,
      "timeid.hour": 10,
      "lasttime": ISODate("2024-01-15T10:05:30Z"),
      
      // Identifiers
      "total.identifier": "total",
      "total.humanidentifier": "total",
      "apiid.api123.identifier": "api123",
      "apiid.api123.humanidentifier": "User API",
      "apikeys.key123.identifier": "key123",
      "apikeys.key123.humanidentifier": "John Doe",
      "endpoints./users/123.identifier": "/users/123",
      "endpoints./users/123.humanidentifier": "/users/123",
      
      // Network stats
      "total.openconnections": 5,
      "total.closedconnections": 3,
      "total.bytesin": 1024,
      "total.bytesout": 2048,
      "apiid.api123.openconnections": 5,
      "apiid.api123.closedconnections": 3,
      "apiid.api123.bytesin": 1024,
      "apiid.api123.bytesout": 2048
    },
    "$max": {
      // Maximum latency values
      "total.maxlatency": 150,
      "total.maxupstreamlatency": 100,
      "apiid.api123.maxlatency": 150,
      "apiid.api123.maxupstreamlatency": 100,
      "apikeys.key123.maxlatency": 150,
      "apikeys.key123.maxupstreamlatency": 100,
      "endpoints./users/123.maxlatency": 150,
      "endpoints./users/123.maxupstreamlatency": 100,
      "keyendpoints.key123./users/123.maxlatency": 150,
      "keyendpoints.key123./users/123.maxupstreamlatency": 100
    },
    "$min": {
      // Minimum latency values (only for successful requests)
      "total.minlatency": 150,
      "total.minupstreamlatency": 100,
      "apiid.api123.minlatency": 150,
      "apiid.api123.minupstreamlatency": 100,
      "apikeys.key123.minlatency": 150,
      "apikeys.key123.minupstreamlatency": 100,
      "endpoints./users/123.minlatency": 150,
      "endpoints./users/123.minupstreamlatency": 100,
      "keyendpoints.key123./users/123.minlatency": 150,
      "keyendpoints.key123./users/123.minupstreamlatency": 100
    }
  },
  { upsert: true }
)
```

## Phase 2: Average Calculations (AsTimeUpdate)

### Basic Structure
```javascript
db.z_tyk_analyticz_aggregate_org123.updateOne(
  {
    "orgid": "org123",
    "timestamp": ISODate("2024-01-15T10:00:00Z")
  },
  {
    "$set": {
      // Average calculations
      // Lists for top-N analytics
    }
  },
  { upsert: true }
)
```

### Complete Phase 2 Example

```javascript
db.z_tyk_analyticz_aggregate_org123.updateOne(
  {
    "orgid": "org123",
    "timestamp": ISODate("2024-01-15T10:00:00Z")
  },
  {
    "$set": {
      // Average request times
      "total.requesttime": 150.5,
      "apiid.api123.requesttime": 150.5,
      "apikeys.key123.requesttime": 150.5,
      "endpoints./users/123.requesttime": 150.5,
      "keyendpoints.key123./users/123.requesttime": 150.5,
      
      // Average latency calculations
      "total.latency": 150.0,
      "total.upstreamlatency": 100.0,
      "apiid.api123.latency": 150.0,
      "apiid.api123.upstreamlatency": 100.0,
      "apikeys.key123.latency": 150.0,
      "apikeys.key123.upstreamlatency": 100.0,
      "endpoints./users/123.latency": 150.0,
      "endpoints./users/123.upstreamlatency": 100.0,
      "keyendpoints.key123./users/123.latency": 150.0,
      "keyendpoints.key123./users/123.upstreamlatency": 100.0,
      
      // Error lists
      "total.errorlist": [
        { "code": "404", "count": 5 },
        { "code": "500", "count": 2 }
      ],
      "apiid.api123.errorlist": [
        { "code": "404", "count": 3 },
        { "code": "500", "count": 1 }
      ],
      
      // Lists for dashboard analytics
      "lists.apiid": [
        {
          "hits": 100,
          "success": 95,
          "error": 5,
          "requesttime": 150.5,
          "latency": 150.0,
          "upstreamlatency": 100.0,
          "identifier": "api123",
          "humanidentifier": "User API",
          "errorlist": [
            { "code": "404", "count": 3 },
            { "code": "500", "count": 2 }
          ]
        }
      ],
      "lists.errors": [
        {
          "hits": 5,
          "error": 5,
          "requesttime": 200.0,
          "latency": 180.0,
          "upstreamlatency": 120.0,
          "identifier": "404",
          "humanidentifier": "404",
          "errorlist": [
            { "code": "404", "count": 5 }
          ]
        }
      ],
      "lists.apikeys": [
        {
          "hits": 50,
          "success": 48,
          "error": 2,
          "requesttime": 145.0,
          "latency": 145.0,
          "upstreamlatency": 95.0,
          "identifier": "key123",
          "humanidentifier": "John Doe",
          "errorlist": [
            { "code": "404", "count": 2 }
          ]
        }
      ],
      "lists.endpoints": [
        {
          "hits": 25,
          "success": 24,
          "error": 1,
          "requesttime": 155.0,
          "latency": 155.0,
          "upstreamlatency": 105.0,
          "identifier": "/users/123",
          "humanidentifier": "/users/123",
          "errorlist": [
            { "code": "404", "count": 1 }
          ]
        }
      ],
      "lists.keyendpoints": {
        "key123": [
          {
            "hits": 25,
            "success": 24,
            "error": 1,
            "requesttime": 155.0,
            "latency": 155.0,
            "upstreamlatency": 105.0,
            "identifier": "/users/123",
            "humanidentifier": "/users/123",
            "errorlist": [
              { "code": "404", "count": 1 }
            ]
          }
        ]
      }
    }
  },
  { upsert: true }
)
```

## Error Handling Examples

### Document Size Limit Error
```javascript
// When document exceeds 16MB, self-healing kicks in
// Error message: "Size must be between 0 and 16777216"
// Solution: Reduce aggregation time and create new document
```

### Error Map Increments
```javascript
// For HTTP 404 errors
"$inc": {
  "errors.404.hits": 1,
  "errors.404.errortotal": 1,
  "errors.404.errormap.404": 1
}

// For HTTP 500 errors  
"$inc": {
  "errors.500.hits": 1,
  "errors.500.errortotal": 1,
  "errors.500.errormap.500": 1
}
```

## Special Cases

### GraphQL Analytics
```javascript
// For GraphQL records, additional dimensions are tracked
"types.Query.hits": 1,
"types.Mutation.hits": 1,
"fields.user.id.hits": 1,
"operation.getUser.hits": 1,
"rootfields.user.hits": 1
```

### OAuth Analytics
```javascript
// OAuth-specific counters
"oauthids.oauth123.hits": 1,
"oauthids.oauth123.success": 1,
"oauthendpoints.oauth123./users/123.hits": 1
```

### Geographic Analytics
```javascript
// Geo-based counters
"geo.US.hits": 1,
"geo.US.success": 1,
"geo.US.identifier": "US",
"geo.US.humanidentifier": "United States"
```

### Tag-based Analytics
```javascript
// Tag counters (with prefix filtering)
"tags.production.hits": 1,
"tags.production.success": 1,
"tags.production.identifier": "production",
"tags.production.humanidentifier": "production"
```

## Performance Optimizations

### Index Usage
```javascript
// Ensure these indexes exist for optimal performance
db.z_tyk_analyticz_aggregate_org123.createIndex(
  {"orgid": 1, "timestamp": 1}, 
  {background: true}
)

db.z_tyk_analyticz_aggregate_org123.createIndex(
  {"expireAt": 1}, 
  {expireAfterSeconds: 0, background: true}
)
```