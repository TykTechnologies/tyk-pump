package serializer

import (
	"github.com/TykTechnologies/tyk-pump/analytics"
	analyticsproto "github.com/TykTechnologies/tyk-pump/serializer/analytics"
	"github.com/golang/protobuf/proto"
	"github.com/jinzhu/copier"
	"time"
)

type ProtobufSerializer struct {
}

func (pb *ProtobufSerializer) Encode(record *analytics.AnalyticsRecord) ([]byte, error) {
	protoRecord := pb.TransfromSingleRecordToProto(*record)
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

	newRec := analyticsproto.AnalyticsRecord{}
	copier.Copy(&newRec, &rec)

	// get original huso horario
	// grab the ms
	// if huso horario != utc then convert
	// sumar ms

	TimestampToProto(&newRec, rec)

	return newRec
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
