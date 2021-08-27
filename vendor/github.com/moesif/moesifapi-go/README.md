# MoesifApi Lib for Golang

Send REST API Events to Moesif for API analytics and monitoring.

[Source Code on GitHub](https://github.com/moesif/moesifapi-go)

## Introduction

This lib has both synchronous and async methods:

- The synchronous methods call the Moesif API directly
- The async methods (Which start with _QueueXXX_) will queue the requests into batches
and flush buffers periodically.

Async methods are expected to be the common use case

## How to install
Run the following commands:

moesifapi-go can be installed like any other Go library through go get:

```bash
go get github.com/moesif/moesifapi-go
```

Or, if you are already using Go Modules, specify a version number as well:

```bash
go get github.com/moesif/moesifapi-go@v1.0.6
```

## How to use

(See examples/api_test.go for usage examples)

### Create single event

```go
import "github.com/moesif/moesifapi-go"
import "github.com/moesif/moesifapi-go/models"
import "time"

var apiEndpoint string
var batchSize int
var eventQueueSize int 
var timerWakeupSeconds int

apiClient := moesifapi.NewAPI("YOUR_COLLECTOR_APPLICATION_ID", &apiEndpoint, eventQueueSize, batchSize, timerWakeupSeconds)

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
userId := "my_user_id"
companyId := "my_company_id"
metadata := map[string]interface{}{
		"Key1": "metadata",
		"Key2": 12,
		"Key3": map[string]interface{}{
			"Key3_1": "SomeValue",
		},
	}

event := models.EventModel{
  Request:      req,
  Response:     rsp,
  SessionToken: &sessionToken,
  Tags:         nil,
  UserId:       &userId,
  CompanyId: 	&companyId,
  Metadata: 	&metadata,
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

var apiEndpoint string
var batchSize int
var eventQueueSize int 
var timerWakeupSeconds int

apiClient := moesifapi.NewAPI("YOUR_COLLECTOR_APPLICATION_ID", &apiEndpoint, eventQueueSize, batchSize, timerWakeupSeconds)

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
userId := "my_user_id"
companyId := "my_company_id"
metadata := map[string]interface{}{
		"Key1": "metadata",
		"Key2": 12,
		"Key3": map[string]interface{}{
			"Key3_1": "SomeValue",
		},
	}

event := models.EventModel{
  Request:      req,
  Response:     rsp,
  SessionToken: &sessionToken,
  Tags:         nil,
  UserId:       &userId,
  CompanyId: 	&companyId,
  Metadata: 	&metadata,
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

## Update a Single User

Create or update a user profile in Moesif.
The metadata field can be any customer demographic or other info you want to store.
Only the `UserId` field is required.
For details, visit the [Go API Reference](https://www.moesif.com/docs/api?go#update-a-user).

```go
import "github.com/moesif/moesifapi-go"
import "github.com/moesif/moesifapi-go/models"

func literalFieldValue(value string) *string {
    return &value
}

var apiEndpoint string
var batchSize int
var eventQueueSize int 
var timerWakeupSeconds int

apiClient := moesifapi.NewAPI("YOUR_COLLECTOR_APPLICATION_ID", &apiEndpoint, eventQueueSize, batchSize, timerWakeupSeconds)

// Campaign object is optional, but useful if you want to track ROI of acquisition channels
// See https://www.moesif.com/docs/api#users for campaign schema
campaign := models.CampaignModel {
  UtmSource: literalFieldValue("google"),
  UtmMedium: literalFieldValue("cpc"), 
  UtmCampaign: literalFieldValue("adwords"),
  UtmTerm: literalFieldValue("api+tooling"),
  UtmContent: literalFieldValue("landing"),
}

// metadata can be any custom dictionary
metadata := map[string]interface{}{
  "email": "john@acmeinc.com",
  "first_name": "John",
  "last_name": "Doe",
  "title": "Software Engineer",
  "sales_info": map[string]interface{}{
      "stage": "Customer",
      "lifetime_value": 24000,
      "account_owner": "mary@contoso.com",
  },
}

// Only UserId is required
user := models.UserModel{
  UserId:  "12345",
  CompanyId:  literalFieldValue("67890"), // If set, associate user with a company object
  Campaign:  &campaign,
  Metadata:  &metadata,
}

// Queue the user asynchronously
err := apiClient.QueueUser(&user)

// Update the user synchronously
err := apiClient.UpdateUser(&user)
```

## Update Users in Batch
Similar to UpdateUser, but used to update a list of users in one batch. 
Only the `UserId` field is required.
For details, visit the [Go API Reference](https://www.moesif.com/docs/api?go#update-users-in-batch).

```go
import "github.com/moesif/moesifapi-go"
import "github.com/moesif/moesifapi-go/models"

func literalFieldValue(value string) *string {
    return &value
}

var apiEndpoint string
var batchSize int
var eventQueueSize int 
var timerWakeupSeconds int

apiClient := moesifapi.NewAPI("YOUR_COLLECTOR_APPLICATION_ID", &apiEndpoint, eventQueueSize, batchSize, timerWakeupSeconds)

// List of Users
var users []*models.UserModel

// Campaign object is optional, but useful if you want to track ROI of acquisition channels
// See https://www.moesif.com/docs/api#users for campaign schema
campaign := models.CampaignModel {
  UtmSource: literalFieldValue("google"),
  UtmMedium: literalFieldValue("cpc"), 
  UtmCampaign: literalFieldValue("adwords"),
  UtmTerm: literalFieldValue("api+tooling"),
  UtmContent: literalFieldValue("landing"),
}

// metadata can be any custom dictionary
metadata := map[string]interface{}{
  "email": "john@acmeinc.com",
  "first_name": "John",
  "last_name": "Doe",
  "title": "Software Engineer",
  "sales_info": map[string]interface{}{
      "stage": "Customer",
      "lifetime_value": 24000,
      "account_owner": "mary@contoso.com",
  },
}

// Only UserId is required
userA := models.UserModel{
  UserId:  "12345",
  CompanyId:  literalFieldValue("67890"), // If set, associate user with a company object
  Campaign:  &campaign,
  Metadata:  &metadata,
}

users = append(users, &userA)

// Queue the user asynchronously
err := apiClient.QueueUsers(&users)

// Update the user synchronously
err := apiClient.UpdateUsersBatch(&users)
```

## Update a Single Company

Create or update a company profile in Moesif.
The metadata field can be any company demographic or other info you want to store.
Only the `CompanyId` field is required.
For details, visit the [Go API Reference](https://www.moesif.com/docs/api?go#update-a-company).

```go
import "github.com/moesif/moesifapi-go"
import "github.com/moesif/moesifapi-go/models"

func literalFieldValue(value string) *string {
    return &value
}

var apiEndpoint string
var batchSize int
var eventQueueSize int 
var timerWakeupSeconds int

apiClient := moesifapi.NewAPI("YOUR_COLLECTOR_APPLICATION_ID", &apiEndpoint, eventQueueSize, batchSize, timerWakeupSeconds)

// Campaign object is optional, but useful if you want to track ROI of acquisition channels
// See https://www.moesif.com/docs/api#update-a-company for campaign schema
campaign := models.CampaignModel {
  UtmSource: literalFieldValue("google"),
  UtmMedium: literalFieldValue("cpc"), 
  UtmCampaign: literalFieldValue("adwords"),
  UtmTerm: literalFieldValue("api+tooling"),
  UtmContent: literalFieldValue("landing"),
}

// metadata can be any custom dictionary
metadata := map[string]interface{}{
  "org_name": "Acme, Inc",
  "plan_name": "Free",
  "deal_stage": "Lead",
  "mrr": 24000,
  "demographics": map[string]interface{}{
      "alexa_ranking": 500000,
      "employee_count": 47,
  },
}

// Prepare company model
company := models.CompanyModel{
    CompanyId:        "67890",  // The only required field is your company id
    CompanyDomain:    literalFieldValue("acmeinc.com"), // If domain is set, Moesif will enrich your profiles with publicly available info 
    Campaign:         &campaign,
    Metadata:         &metadata,
}

// Queue the company asynchronously
apiClient.QueueCompany(&company)

// Update the company synchronously
err := apiClient.UpdateCompany(&company)
```

## Update Companies in Batch

Similar to updateCompany, but used to update a list of companies in one batch. 
Only the `CompanyId` field is required.
For details, visit the [Go API Reference](https://www.moesif.com/docs/api?go#update-companies-in-batch).

```go
import "github.com/moesif/moesifapi-go"
import "github.com/moesif/moesifapi-go/models"

func literalFieldValue(value string) *string {
    return &value
}

var apiEndpoint string
var batchSize int
var eventQueueSize int 
var timerWakeupSeconds int

apiClient := moesifapi.NewAPI("YOUR_COLLECTOR_APPLICATION_ID", &apiEndpoint, eventQueueSize, batchSize, timerWakeupSeconds)

// List of Companies
var companies []*models.CompanyModel

// Campaign object is optional, but useful if you want to track ROI of acquisition channels
// See https://www.moesif.com/docs/api#update-a-company for campaign schema
campaign := models.CampaignModel {
  UtmSource: literalFieldValue("google"),
  UtmMedium: literalFieldValue("cpc"), 
  UtmCampaign: literalFieldValue("adwords"),
  UtmTerm: literalFieldValue("api+tooling"),
  UtmContent: literalFieldValue("landing"),
}

// metadata can be any custom dictionary
metadata := map[string]interface{}{
  "org_name": "Acme, Inc",
  "plan_name": "Free",
  "deal_stage": "Lead",
  "mrr": 24000,
  "demographics": map[string]interface{}{
      "alexa_ranking": 500000,
      "employee_count": 47,
  },
}

// Prepare company model
companyA := models.CompanyModel{
    CompanyId:        "67890",  // The only required field is your company id
    CompanyDomain:    literalFieldValue("acmeinc.com"), // If domain is set, Moesif will enrich your profiles with publicly available info 
    Campaign:         &campaign,
    Metadata:         &metadata,
}

companies = append(companies, &companyA)

// Queue the company asynchronously
apiClient.QueueCompanies(&companies)

// Update the company synchronously
err := apiClient.UpdateCompaniesBatch(&companies)
```

### Health Check

```bash
go get github.com/moesif/moesifapi-go/health;
```

## How To Test:
```bash
git clone https://github.com/moesif/moesifapi-go
cd moesifapi-go/examples
# Be sure to edit the examples/api_test.go to change the application id to your real one obtained from Moesif.
# var applicationId = "Your Moesif Application Id"
go test  -v
```


## Other integrations

To view more documentation on integration options, please visit __[the Integration Options Documentation](https://www.moesif.com/docs/getting-started/integration-options/).__
