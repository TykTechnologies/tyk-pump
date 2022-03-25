package serializer

import (
	"github.com/TykTechnologies/tyk-pump/analytics"
	analyticsproto "github.com/TykTechnologies/tyk-pump/serializer/analytics"
	"github.com/golang/protobuf/proto"
	"github.com/jinzhu/copier"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type ProtobufSerializer struct {
}

func (pb *ProtobufSerializer) Encode(record *analytics.AnalyticsRecord) ([]byte, error) {
	protoRecord := pb.TransfromSingleRecordToProto(*record)

	return proto.Marshal(&protoRecord)
}

func (pb *ProtobufSerializer) Decode(analyticsData interface{}, record *analytics.AnalyticsRecord) error {
	return pb.TransformFromProtoToAnalyticsRecord(analyticsData, record)
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

	newRec.TimeStamp = timestamppb.New(rec.TimeStamp)
	newRec.ExpireAt = timestamppb.New(rec.ExpireAt)

	return newRec
}

func (pb *ProtobufSerializer) TransformFromProtoToAnalyticsRecord(data interface{}, record *analytics.AnalyticsRecord) error {

	analyticsData := data.(analyticsproto.AnalyticsRecord)
	err := copier.Copy(&record, analyticsData)

	record.TimeStamp = analyticsData.TimeStamp.AsTime()
	record.ExpireAt = analyticsData.ExpireAt.AsTime()

	return err
}
