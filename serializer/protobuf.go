package serializer

import (
	"github.com/TykTechnologies/tyk-pump/analytics"
	analyticsproto "github.com/TykTechnologies/tyk-pump/serializer/analytics"
	"github.com/golang/protobuf/proto"
	"github.com/jinzhu/copier"
	"google.golang.org/protobuf/types/known/timestamppb"
	"time"
)

type ProtobufSerializer struct {
}

func (pb *ProtobufSerializer) Encode(record *analytics.AnalyticsRecord) ([]byte, error) {
	protoRecord := pb.TransfromSingleRecordToProto(*record)

	log.Info("timezone:" + protoRecord.TimeZone)
	return proto.Marshal(&protoRecord)
}

func (pb *ProtobufSerializer) Decode(analyticsData interface{}, record *analytics.AnalyticsRecord) error {
	protoData := analyticsproto.AnalyticsRecord{}
	err := proto.Unmarshal(analyticsData.([]byte), &protoData)
	if err != nil {
		return err
	}
	return pb.TransformFromProtoToAnalyticsRecord(protoData, record)
}

func (pb *ProtobufSerializer) GetSuffix() string {
	return "_protobuf"
}

func (pb *ProtobufSerializer) TransformToProto(recs []analytics.AnalyticsRecord) []analyticsproto.AnalyticsRecord {
	transformedRecs := make([]analyticsproto.AnalyticsRecord, len(recs))

	for i, _ := range recs {
		transformedRecs[i] = pb.TransfromSingleRecordToProto(recs[i])
	}
	return transformedRecs
}

func (pb *ProtobufSerializer) TransfromSingleRecordToProto(rec analytics.AnalyticsRecord) analyticsproto.AnalyticsRecord {
	newRec := AnalyticsRecordToProto(rec)
	return newRec
}

func AnalyticsRecordToProto(rec analytics.AnalyticsRecord) analyticsproto.AnalyticsRecord {
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
			Names: rec.Geo.City.Names,
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
	}

	TimestampToProto(&record, rec)

	return record
}

func (pb *ProtobufSerializer) TransformFromProtoToAnalyticsRecord(protoRecord analyticsproto.AnalyticsRecord, record *analytics.AnalyticsRecord) error {

	err := copier.Copy(&record, protoRecord)
	TimeStampFromProto(protoRecord, record)

	return err
}

// TimestampToProto will process timestamps and assign them to the proto record
// protobuf converts all timestamps to UTC so we need to ensure that we keep
// the same original location, in order to do so, we store the location
func TimestampToProto(newRecord *analyticsproto.AnalyticsRecord, record analytics.AnalyticsRecord) {
	// save original location
	newRecord.TimeStamp = timestamppb.New(record.TimeStamp)
	newRecord.ExpireAt = timestamppb.New(record.ExpireAt)
	newRecord.TimeZone = record.TimeStamp.Location().String()
}

func TimeStampFromProto(protoRecord analyticsproto.AnalyticsRecord, record *analytics.AnalyticsRecord) {
	// get timestamp in original location
	loc, err := time.LoadLocation(protoRecord.TimeZone)
	if err != nil {
		log.Error(err)
		return
	}

	// assign timestamp in original location
	record.TimeStamp = protoRecord.TimeStamp.AsTime().In(loc)
	record.ExpireAt = protoRecord.ExpireAt.AsTime().In(loc)
}
