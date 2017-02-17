# MoesifApi Lib for Golang

Send REST API Events to Moesif for error analysis

[Source Code on GitHub](https://github.com/moesif/moesifapi-go)

## Introduction

This lib has both synchronous and async methods:

- The synchronous methods call the Moesif API directly
- The async methods (Which start with _QueueXXX_) will queue the requests into batches
and flush buffers periodically.

Async methods are expected to be the common use case

## How to install
Run the following commands:

```bash
go get github.com/moesif/moesifapi-go;
```

## How to use

(See examples/api_test.go for usage examples)

### Create single event

```go
import "github.com/moesif/moesifapi-go"
import "github.com/moesif/moesifapi-go/models"
import "time"

apiClient := moesifapi.NewAPI("my_application_id")

reqTime := time.Now().UTC()
apiVersion := "1.0"
ipAddress := "61.48.220.123"

req := models.EventRequestModel{
	Time:       &reqTime,
	Uri:        "https://api.acmeinc.com/widgets",
	Verb:       "GET",
	ApiVersion: &apiVersion,
	IpAddress:  &ipAddress,
	Headers: map[string]interface{}{
		"ReqHeader1": "ReqHeaderValue1",
	},
	Body: nil,
}

rspTime := time.Now().UTC().Add(time.Duration(1) * time.Second)

rsp := models.EventResponseModel{
	Time:      &rspTime,
	Status:    500,
	IpAddress: nil,
	Headers: map[string]interface{}{
		"RspHeader1": "RspHeaderValue1",
	},
	Body: map[string]interface{}{
		"Key1": "Value1",
		"Key2": 12,
		"Key3": map[string]interface{}{
			"Key3_1": "SomeValue",
		},
	},
}

sessionToken := "23jdf0owekfmcn4u3qypxg09w4d8ayrcdx8nu2ng]s98y18cx98q3yhwmnhcfx43f"
userId := "end_user_id"

event := models.EventModel{
	Request:      req,
	Response:     rsp,
	SessionToken: &sessionToken,
	Tags:         nil,
	UserId:       &userId,
}

// Queue the events
err := apiClient.QueueEvent(&event)

// Create the events synchronously
err := apiClient.CreateEvent(&event)

```

### Create batches of events with bulk API


```go
import "github.com/moesif/moesifapi-go"
import "github.com/moesif/moesifapi-go/models"
import "time"

apiClient := moesifapi.NewAPI("my_application_id")

reqTime := time.Now().UTC()
apiVersion := "1.0"
ipAddress := "61.48.220.123"

req := models.EventRequestModel{
	Time:       &reqTime,
	Uri:        "https://api.acmeinc.com/widgets",
	Verb:       "GET",
	ApiVersion: &apiVersion,
	IpAddress:  &ipAddress,
	Headers: map[string]interface{}{
		"ReqHeader1": "ReqHeaderValue1",
	},
	Body: nil,
}

rspTime := time.Now().UTC().Add(time.Duration(1) * time.Second)

rsp := models.EventResponseModel{
	Time:      &rspTime,
	Status:    500,
	IpAddress: nil,
	Headers: map[string]interface{}{
		"RspHeader1": "RspHeaderValue1",
	},
	Body: map[string]interface{}{
		"Key1": "Value1",
		"Key2": 12,
		"Key3": map[string]interface{}{
			"Key3_1": "SomeValue",
		},
	},
}

sessionToken := "23jdf0owekfmcn4u3qypxg09w4d8ayrcdx8nu2ng]s98y18cx98q3yhwmnhcfx43f"
userId := "end_user_id"

event := models.EventModel{
	Request:      req,
	Response:     rsp,
	SessionToken: &sessionToken,
	Tags:         nil,
	UserId:       &userId,
}

events := make([]*models.EventModel, 20)
for i := 0; i < 20; i++ {
	events[i] = &event
}

// Queue the events
err := apiClient.QueueEvents(events)

// Create the events batch synchronously
err := apiClient.CreateEventsBatch(event)

```

### Health Check

```bash
go get github.com/moesif/moesifapi-go/health;
```

## How To Test:
```bash
git clone https://github.com/moesif/moesifapi-go
cd moesifapi-go/examples
go test  -v
```


## Other integrations

To view more more documentation on integration options, please visit __[the Integration Options Documentation](https://www.moesif.com/docs/getting-started/integration-options/).__
