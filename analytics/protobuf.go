package analytics

import (
	"time"

	analyticsproto "github.com/TykTechnologies/tyk-pump/analytics/proto"
	"github.com/golang/protobuf/proto"
)

type ProtobufSerializer struct {
}

func (pb *ProtobufSerializer) GetSuffix() string {
	return "_protobuf"
}

func (pb *ProtobufSerializer) Encode(record *AnalyticsRecord) ([]byte, error) {
	protoRecord := pb.TransformSingleRecordToProto(*record)
	return proto.Marshal(&protoRecord)
}

func (pb *ProtobufSerializer) Decode(analyticsData interface{}, record *AnalyticsRecord) error {
	protoData := analyticsproto.AnalyticsRecord{}
	err := proto.Unmarshal(analyticsData.([]byte), &protoData)
	if err != nil {
		return err
	}
	return pb.TransformSingleProtoToAnalyticsRecord(protoData, record)
}

func (pb *ProtobufSerializer) TransformSingleRecordToProto(rec AnalyticsRecord) analyticsproto.AnalyticsRecord {
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

	return record
}

func (pb *ProtobufSerializer) TransformSingleProtoToAnalyticsRecord(rec analyticsproto.AnalyticsRecord, record *AnalyticsRecord) error {

	tmpRecord := AnalyticsRecord{
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
		Geo: GeoData{
			Country: Country{
				ISOCode: rec.Geo.Country.ISOCode,
			},
			City: City{
				GeoNameID: uint(rec.Geo.City.GeoNameID),
				Names:     nil,
			},
			Location: Location{
				Latitude:  rec.Geo.Location.Latitude,
				Longitude: rec.Geo.Location.Longitude,
				TimeZone:  rec.Geo.Location.TimeZone,
			},
		},
		Network: NetworkStats{
			OpenConnections:  rec.Network.OpenConnections,
			ClosedConnection: rec.Network.ClosedConnections,
			BytesIn:          rec.Network.BytesIn,
			BytesOut:         rec.Network.BytesOut,
		},
		Latency: Latency{
			Total:    rec.Latency.Total,
			Upstream: rec.Latency.Upstream,
		},
		Tags:      rec.Tags,
		Alias:     rec.Alias,
		TrackPath: rec.TrackPath,
		ApiSchema: rec.ApiSchema,
	}
	tmpRecord.TimeStampFromProto(rec)
	*record = tmpRecord
	return nil
}
