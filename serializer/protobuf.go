package serializer

import (
	"time"

	"github.com/TykTechnologies/tyk-pump/analytics"
	analyticsproto "github.com/TykTechnologies/tyk-pump/analytics/proto"
	"github.com/golang/protobuf/proto"
)

type ProtobufSerializer struct {
}

func (pb *ProtobufSerializer) GetSuffix() string {
	return "_protobuf"
}

func (pb *ProtobufSerializer) Encode(record *analytics.AnalyticsRecord) ([]byte, error) {
	protoRecord := pb.TransformSingleRecordToProto(*record)
	return proto.Marshal(&protoRecord)
}

func (pb *ProtobufSerializer) Decode(analyticsData interface{}, record *analytics.AnalyticsRecord) error {
	protoData := analyticsproto.AnalyticsRecord{}
	err := proto.Unmarshal(analyticsData.([]byte), &protoData)
	if err != nil {
		return err
	}
	return pb.TransformSingleProtoToAnalyticsRecord(protoData, record)
}

func (pb *ProtobufSerializer) TransformSingleRecordToProto(rec analytics.AnalyticsRecord) analyticsproto.AnalyticsRecord {
	latency := analyticsproto.Latency{
		Total:    rec.Latency.Total,
		Upstream: rec.Latency.Upstream,
	}

	net := analyticsproto.NetworkStats{
		OpenConnections:   rec.Network.OpenConnections,
		ClosedConnections: rec.Network.ClosedConnection,
		BytesIn:           rec.Network.BytesIn,
		BytesOut:          rec.Network.BytesOut,
	}

	geo := analyticsproto.GeoData{
		Country: &analyticsproto.Country{
			ISOCode: rec.Geo.Country.ISOCode,
		},
		City: &analyticsproto.City{
			GeoNameID: uint32(rec.Geo.City.GeoNameID),
			Names:     rec.Geo.City.Names,
		},
		Location: &analyticsproto.Location{
			Latitude:  rec.Geo.Location.Latitude,
			Longitude: rec.Geo.Location.Longitude,
			TimeZone:  rec.Geo.Location.TimeZone,
		},
	}

	record := analyticsproto.AnalyticsRecord{
		Host:          rec.Host,
		Method:        rec.Method,
		Path:          rec.Path,
		RawPath:       rec.RawPath,
		ContentLength: rec.ContentLength,
		UserAgent:     rec.UserAgent,
		Day:           int32(rec.Day),
		Month:         int32(rec.Month),
		Year:          int32(rec.Year),
		Hour:          int32(rec.Hour),
		ResponseCode:  int32(rec.ResponseCode),
		APIKey:        rec.APIKey,
		APIVersion:    rec.APIVersion,
		APIName:       rec.APIName,
		APIID:         rec.APIID,
		OrgID:         rec.OrgID,
		RequestTime:   rec.RequestTime,
		Latency:       &latency,
		RawRequest:    rec.RawRequest,
		RawResponse:   rec.RawResponse,
		IPAddress:     rec.IPAddress,
		Geo:           &geo,
		Network:       &net,
		Tags:          rec.Tags,
		Alias:         rec.Alias,
		TrackPath:     rec.TrackPath,
		OauthID:       rec.OauthID,
		ApiSchema:     rec.ApiSchema,
	}
	rec.TimestampToProto(&record)
	if rec.GraphQLStats.IsGraphQL {
		// operation type
		operationType := analyticsproto.GraphQLOperations_OPERATION_UNKNOWN
		switch rec.GraphQLStats.OperationType {
		case analytics.OperationQuery:
			operationType = analyticsproto.GraphQLOperations_OPERATION_QUERY
		case analytics.OperationMutation:
			operationType = analyticsproto.GraphQLOperations_OPERATION_MUTATION
		case analytics.OperationSubscription:
			operationType = analyticsproto.GraphQLOperations_OPERATION_SUBSCRIPTION
		}
		// graph errors
		graphErrors := make([]string, len(rec.GraphQLStats.Errors))
		for i, val := range rec.GraphQLStats.Errors {
			graphErrors[i] = val.Message
		}
		// types
		graphTypes := make(map[string]*analyticsproto.RepeatedFields)
		for key, val := range rec.GraphQLStats.Types {
			graphTypes[key] = &analyticsproto.RepeatedFields{Fields: val}
		}
		record.GraphQLStats = &analyticsproto.GraphQLStats{
			IsGraphQL:     true,
			Variables:     rec.GraphQLStats.Variables,
			HasError:      rec.GraphQLStats.HasErrors,
			OperationType: operationType,
			GraphErrors:   graphErrors,
			RootFields:    rec.GraphQLStats.RootFields,
			Types:         graphTypes,
		}
	}

	return record
}

func (pb *ProtobufSerializer) TransformSingleProtoToAnalyticsRecord(rec analyticsproto.AnalyticsRecord, record *analytics.AnalyticsRecord) error {
	tmpRecord := analytics.AnalyticsRecord{
		Method:        rec.Method,
		Host:          rec.Host,
		Path:          rec.Path,
		RawPath:       rec.RawPath,
		ContentLength: rec.ContentLength,
		UserAgent:     rec.UserAgent,
		Day:           int(rec.Day),
		Month:         time.Month(rec.Month),
		Year:          int(rec.Year),
		Hour:          int(rec.Hour),
		ResponseCode:  int(rec.ResponseCode),
		APIKey:        rec.APIKey,
		APIVersion:    rec.APIVersion,
		APIName:       rec.APIName,
		APIID:         rec.APIID,
		OrgID:         rec.OrgID,
		OauthID:       rec.OauthID,
		RequestTime:   rec.RequestTime,
		RawRequest:    rec.RawRequest,
		RawResponse:   rec.RawResponse,
		IPAddress:     rec.IPAddress,
		Geo: analytics.GeoData{
			Country: analytics.Country{
				ISOCode: rec.Geo.Country.ISOCode,
			},
			City: analytics.City{
				GeoNameID: uint(rec.Geo.City.GeoNameID),
				Names:     nil,
			},
			Location: analytics.Location{
				Latitude:  rec.Geo.Location.Latitude,
				Longitude: rec.Geo.Location.Longitude,
				TimeZone:  rec.Geo.Location.TimeZone,
			},
		},
		Network: analytics.NetworkStats{
			OpenConnections:  rec.Network.OpenConnections,
			ClosedConnection: rec.Network.ClosedConnections,
			BytesIn:          rec.Network.BytesIn,
			BytesOut:         rec.Network.BytesOut,
		},
		Latency: analytics.Latency{
			Total:    rec.Latency.Total,
			Upstream: rec.Latency.Upstream,
		},
		Tags:      rec.Tags,
		Alias:     rec.Alias,
		TrackPath: rec.TrackPath,
		ApiSchema: rec.ApiSchema,
	}
	tmpRecord.TimeStampFromProto(rec)

	if rec.GraphQLStats != nil {
		// process anc convert graphql stats
		var operationType analytics.GraphQLOperations
		switch rec.GraphQLStats.OperationType {
		case analyticsproto.GraphQLOperations_OPERATION_QUERY:
			operationType = analytics.OperationQuery
		case analyticsproto.GraphQLOperations_OPERATION_MUTATION:
			operationType = analytics.OperationMutation
		case analyticsproto.GraphQLOperations_OPERATION_SUBSCRIPTION:
			operationType = analytics.OperationSubscription
		default:
			operationType = analytics.OperationUnknown
		}

		types := make(map[string][]string)
		for key, val := range rec.GraphQLStats.Types {
			types[key] = val.Fields
		}
		errors := make([]analytics.GraphError, len(rec.GraphQLStats.GraphErrors))
		for i, val := range rec.GraphQLStats.GraphErrors {
			errors[i].Message = val
		}

		tmpRecord.GraphQLStats = analytics.GraphQLStats{
			IsGraphQL:     rec.GraphQLStats.IsGraphQL,
			OperationType: operationType,
			HasErrors:     rec.GraphQLStats.HasError,
			RootFields:    rec.GraphQLStats.RootFields,
			Types:         types,
			Variables:     rec.GraphQLStats.Variables,
			Errors:        errors,
		}
	}

	*record = tmpRecord
	return nil
}
