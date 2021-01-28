package kafka

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/TykTechnologies/tyk-pump/analytics"
	"github.com/TykTechnologies/tyk-pump/logger"
	"github.com/TykTechnologies/tyk-pump/pumps/kafka/mocks"
	"github.com/segmentio/kafka-go"
	"github.com/stretchr/testify/assert"
)

func getDefaultKafkaConf() KafkaConf{
	return KafkaConf{
		MetaData: map[string]string{"key":"value"},
	}
}

func TestWriteData(t *testing.T){
	keys := make([]interface{}, 1)
	record :=  analytics.AnalyticsRecord{APIID: "api111"}
	keys[0] = record


	jsonRecord := fromRecordToJson(record)
	byteRecord, _ := json.Marshal(jsonRecord)
	messages := []kafka.Message{{
		Time: timeNow(),
		Value: byteRecord,
	}}

	client := &mocks.KafkaClient{}
	logger :=  logger.GetLogger().WithField("prefix", kafkaPrefix)
	pump := KafkaPump{kafkaClient: client, log:logger, kafkaConf: &KafkaConf{}}

	//Testing if everything is ok when we do a normal write
	ctx := context.TODO()
	client.On("WriteMessages",ctx, messages[0]).Return(nil)
	err := pump.WriteData(ctx,keys)
	assert.Equal(t, err , nil)

	//Testing kafka error when context is canceled.
	ctxCancelled, cancel := context.WithCancel(context.TODO())
	cancel()
	client.On("WriteMessages",ctxCancelled, messages[0]).Return(errors.New("context canceled"))
	err = pump.WriteData(ctxCancelled,keys)
	assert.NotEqual(t, err , nil)


	client.AssertExpectations(t)
}

func init(){
	timeNow = func() time.Time {
		 t, _ := time.Parse("2006-01-02 15:04:05", "2017-01-20 01:02:03")
		 return t
	}
}