package pumps

import (
	"bufio"
	"encoding/base64"
	"fmt"
	"strings"
	"time"

	"github.com/mitchellh/mapstructure"
	"github.com/moesif/moesifapi-go"
	"github.com/moesif/moesifapi-go/models"

	"github.com/TykTechnologies/logrus"
	"github.com/TykTechnologies/tyk-pump/analytics"
)

type MoesifPump struct {
	moesifApi  moesifapi.API
	moesifConf *MoesifConf
}

type RawDecoded struct {
	headers map[string]interface{}
	body    interface{}
}

var moesifPrefix string = "moesif-pump"

type MoesifConf struct {
	ApplicationId string `mapstructure:"application_id"`
}

func (e *MoesifPump) New() Pump {
	newPump := MoesifPump{}
	return &newPump
}

func (p *MoesifPump) GetName() string {
	return "Moesif Pump"
}

func (p *MoesifPump) Init(config interface{}) error {
	p.moesifConf = &MoesifConf{}
	loadConfigErr := mapstructure.Decode(config, &p.moesifConf)

	if loadConfigErr != nil {
		log.WithFields(logrus.Fields{
			"prefix": moesifPrefix,
		}).Fatal("Failed to decode configuration: ", loadConfigErr)
	}

	api := moesifapi.NewAPI(p.moesifConf.ApplicationId)
	p.moesifApi = api

	return nil
}

func (p *MoesifPump) WriteData(data []interface{}) error {
	log.WithFields(logrus.Fields{
		"prefix": moesifPrefix,
	}).Info("Writing ", len(data), " records")

	if len(data) == 0 {
		return nil
	}

	transferEncoding := "base64"
	for dataIndex := range data {
		var record, _ = data[dataIndex].(analytics.AnalyticsRecord)

		rawReq, err := base64.StdEncoding.DecodeString(record.RawRequest)
		if err != nil {
			log.WithFields(logrus.Fields{
				"prefix": moesifPrefix,
			}).Fatal(err)
		}

		decodedReqBody, err := decodeRawData(string(rawReq))

		if err != nil {
			log.WithFields(logrus.Fields{
				"prefix": moesifPrefix,
			}).Fatal(err)
		}

		req := models.EventRequestModel{
			Time:             &record.TimeStamp,
			Uri:              record.Path,
			Verb:             record.Method,
			ApiVersion:       &record.APIVersion,
			IpAddress:        &record.IPAddress,
			Headers:          decodedReqBody.headers,
			Body:             &decodedReqBody.body,
			TransferEncoding: &transferEncoding,
		}

		rawRsp, err := base64.StdEncoding.DecodeString(record.RawResponse)

		if err != nil {
			log.WithFields(logrus.Fields{
				"prefix": moesifPrefix,
			}).Fatal(err)
		}

		decodedRspBody, err := decodeRawData(string(rawRsp))

		if err != nil {
			log.WithFields(logrus.Fields{
				"prefix": moesifPrefix,
			}).Fatal(err)
		}

		rspTime := record.TimeStamp.Add(time.Duration(record.RequestTime) * time.Millisecond)

		rsp := models.EventResponseModel{
			Time:             &rspTime,
			Status:           record.ResponseCode,
			IpAddress:        nil,
			Headers:          decodedRspBody.headers,
			Body:             decodedRspBody.body,
			TransferEncoding: &transferEncoding,
		}

		event := models.EventModel{
			Request:      req,
			Response:     rsp,
			SessionToken: &record.APIKey,
			Tags:         nil,
			UserId:       nil,
		}

		err = p.moesifApi.QueueEvent(&event)
		if err != nil {
			log.WithFields(logrus.Fields{
				"prefix": moesifPrefix,
			}).Error("Error while writing ", data[dataIndex], err)
		}
	}

	return nil
}

func decodeRawData(raw string) (*RawDecoded, error) {
	headersBody := strings.SplitN(raw, "\r\n\r\n", 2)

	if len(headersBody) == 0 {
		return nil, fmt.Errorf("Error while splitting raw data")
	}

	headers := decodeHeaders(headersBody[0])

	var body interface{}
	if len(headersBody) == 2 {
		body = base64.StdEncoding.EncodeToString([]byte(headersBody[1]))
	}

	ret := &RawDecoded{
		headers: headers,
		body:    body,
	}

	return ret, nil
}

func decodeHeaders(headers string) map[string]interface{} {

	scanner := bufio.NewScanner(strings.NewReader(headers))
	ret := make(map[string]interface{}, strings.Count(headers, "\r\n"))

	// Remove Request Line or Status Line
	scanner.Scan()
	scanner.Text()

	for scanner.Scan() {
		kv := strings.SplitN(scanner.Text(), ":", 2)

		if len(kv) != 2 {
			continue
		}
		ret[kv[0]] = strings.TrimSpace(kv[1])
	}

	return ret
}
