package pumps

import (
	"bufio"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"github.com/TykTechnologies/logrus"
	"github.com/TykTechnologies/tyk-pump/analytics"
	"github.com/mitchellh/mapstructure"
	"github.com/moesif/moesifapi-go"
	"github.com/moesif/moesifapi-go/models"
	"io/ioutil"
	"math"
	"math/rand"
	"net/http"
	"strconv"
	"strings"
	"time"
)

type MoesifPump struct {
	moesifAPI            moesifapi.API
	moesifConf           *MoesifConf
	filters              analytics.AnalyticsFilters
	timeout              int
	samplingPercentage   int
	eTag                 string
	lastUpdatedTime      time.Time
	appConfig            map[string]interface{}
	userSampleRateMap    map[string]interface{}
	companySampleRateMap map[string]interface{}
	CommonPumpConfig
}

type rawDecoded struct {
	headers map[string]interface{}
	body    interface{}
}

var moesifPrefix = "moesif-pump"

type MoesifConf struct {
	ApplicationID              string   `mapstructure:"application_id"`
	RequestHeaderMasks         []string `mapstructure:"request_header_masks"`
	ResponseHeaderMasks        []string `mapstructure:"response_header_masks"`
	RequestBodyMasks           []string `mapstructure:"request_body_masks"`
	ResponseBodyMasks          []string `mapstructure:"response_body_masks"`
	DisableCaptureRequestBody  bool     `mapstructure:"disable_capture_request_body"`
	DisableCaptureResponseBody bool     `mapstructure:"disable_capture_response_body"`
	UserIDHeader               string   `mapstructure:"user_id_header"`
	CompanyIDHeader            string   `mapstructure:"company_id_header"`
}

func (p *MoesifPump) New() Pump {
	newPump := MoesifPump{}
	return &newPump
}

func (p *MoesifPump) GetName() string {
	return "Moesif Pump"
}

func (p *MoesifPump) parseConfiguration(response *http.Response) (int, string, time.Time) {
	// Get X-Moesif-Config-Etag header from response
	if configETag, ok := response.Header["X-Moesif-Config-Etag"]; ok {
		p.eTag = configETag[0]
	}

	// Read the response body
	respBody, err := ioutil.ReadAll(response.Body)
	if err != nil {
		log.WithFields(logrus.Fields{
			"prefix": moesifPrefix,
		}).Fatal("Couldn't parse configuration: ", err)
		return p.samplingPercentage, p.eTag, time.Now().UTC()
	}
	// Parse the response Body
	if jsonRespParseErr := json.Unmarshal(respBody, &p.appConfig); jsonRespParseErr == nil {
		// Fetch sample rate from appConfig
		if getSampleRate, found := p.appConfig["sample_rate"]; found {
			if rate, ok := getSampleRate.(float64); ok {
				p.samplingPercentage = int(rate)
			}
		}
		// Fetch User Sample rate from appConfig
		if userSampleRate, ok := p.appConfig["user_sample_rate"]; ok {
			if userRates, ok := userSampleRate.(map[string]interface{}); ok {
				p.userSampleRateMap = userRates
			}
		}
		// Fetch Company Sample rate from appConfig
		if companySampleRate, ok := p.appConfig["company_sample_rate"]; ok {
			if companyRates, ok := companySampleRate.(map[string]interface{}); ok {
				p.companySampleRateMap = companyRates
			}
		}
	}

	return p.samplingPercentage, p.eTag, time.Now().UTC()
}

func (p *MoesifPump) getSamplingPercentage(userID string, companyID string) int {
	if userID != "" {
		if userRate, ok := p.userSampleRateMap[userID].(float64); ok {
			return int(userRate)
		}
	}

	if companyID != "" {
		if companyRate, ok := p.companySampleRateMap[companyID].(float64); ok {
			return int(companyRate)
		}
	}

	if getSampleRate, found := p.appConfig["sample_rate"]; found {
		if rate, ok := getSampleRate.(float64); ok {
			return int(rate)
		}
	}

	return 100
}

func fetchIDFromHeader(requestHeaders map[string]interface{}, responseHeaders map[string]interface{}, headerName string) string {
	var id string
	if requid, ok := requestHeaders[headerName].(string); ok {
		id = requid
	}
	if resuid, ok := responseHeaders[headerName].(string); ok {
		id = resuid
	}
	return id
}

func toLowerCase(headers map[string]interface{}) map[string]interface{} {
	transformMap := make(map[string]interface{}, len(headers))
	for k, v := range headers {
		transformMap[strings.ToLower(k)] = v
	}
	return transformMap
}

func contains(arr []string, str string) bool {
	for _, value := range arr {
		if value == str {
			return true
		}
	}
	return false
}

func maskData(data map[string]interface{}, maskBody []string) map[string]interface{} {
	for key, val := range data {
		switch val.(type) {
		case map[string]interface{}:
			if contains(maskBody, key) {
				data[key] = "*****"
			} else {
				maskData(val.(map[string]interface{}), maskBody)
			}
		default:
			if contains(maskBody, key) {
				data[key] = "*****"
			}
		}
	}
	return data
}

func maskRawBody(rawBody string, maskBody []string) string {
	// Mask body
	var maskedBody map[string]interface{}
	if err := json.Unmarshal([]byte(rawBody), &maskedBody); err == nil {

		if len(maskBody) > 0 {
			maskedBody = maskData(maskedBody, maskBody)
		}

		out, _ := json.Marshal(maskedBody)
		return base64.StdEncoding.EncodeToString([]byte(out))
	}

	return base64.StdEncoding.EncodeToString([]byte(rawBody))
}

func buildURI(raw string, defaultPath string) string {
	pathHeadersBody := strings.SplitN(raw, "\r\n", 2)

	if len(pathHeadersBody) >= 2 {
		requestPath := strings.Fields(pathHeadersBody[0])
		if len(requestPath) >= 3 {
			url := requestPath[1]
			return url
		}
		return defaultPath
	}
	return defaultPath
}

func (p *MoesifPump) Init(config interface{}) error {
	p.moesifConf = &MoesifConf{}
	loadConfigErr := mapstructure.Decode(config, &p.moesifConf)

	if loadConfigErr != nil {
		log.WithFields(logrus.Fields{
			"prefix": moesifPrefix,
		}).Fatal("Failed to decode configuration: ", loadConfigErr)
	}

	api := moesifapi.NewAPI(p.moesifConf.ApplicationID)
	p.moesifAPI = api

	// Default samplingPercentage and DateTime
	p.samplingPercentage = 100
	p.lastUpdatedTime = time.Now().UTC()

	// Fetch application config
	response, err := p.moesifAPI.GetAppConfig()

	if err == nil {
		p.samplingPercentage, p.eTag, p.lastUpdatedTime = p.parseConfiguration(response)
	} else {
		log.WithFields(logrus.Fields{
			"prefix": moesifPrefix,
		}).Debug("Error fetching application configuration on initilization with err -  " + err.Error())
	}

	return nil
}

func (p *MoesifPump) WriteData(ctx context.Context, data []interface{}) error {
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

		decodedReqBody, err := decodeRawData(string(rawReq), p.moesifConf.RequestHeaderMasks,
			p.moesifConf.RequestBodyMasks, p.moesifConf.DisableCaptureRequestBody)

		if err != nil {
			log.WithFields(logrus.Fields{
				"prefix": moesifPrefix,
			}).Fatal(err)
		}

		// Request URL
		requestURL := buildURI(string(rawReq), record.Path)

		// Request Time
		reqTime := record.TimeStamp.UTC()

		req := models.EventRequestModel{
			Time:             &reqTime,
			Uri:              requestURL,
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

		decodedRspBody, err := decodeRawData(string(rawRsp), p.moesifConf.ResponseHeaderMasks,
			p.moesifConf.ResponseBodyMasks, p.moesifConf.DisableCaptureResponseBody)

		if err != nil {
			log.WithFields(logrus.Fields{
				"prefix": moesifPrefix,
			}).Fatal(err)
		}

		// Response Time
		rspTime := record.TimeStamp.Add(time.Duration(record.RequestTime) * time.Millisecond).UTC()

		rsp := models.EventResponseModel{
			Time:             &rspTime,
			Status:           record.ResponseCode,
			IpAddress:        nil,
			Headers:          decodedRspBody.headers,
			Body:             decodedRspBody.body,
			TransferEncoding: &transferEncoding,
		}

		// Add Metadata
		metadata := map[string]interface{}{
			"tyk": map[string]interface{}{
				"api_name": record.APIName,
				"tags":     record.Tags,
			},
		}

		// Direction to the event
		direction := "Incoming"

		// User Id
		var userID string
		if p.moesifConf.UserIDHeader != "" {
			userID = fetchIDFromHeader(decodedReqBody.headers, decodedRspBody.headers, p.moesifConf.UserIDHeader)
		}

		if userID == "" {
			if record.Alias != "" {
				userID = record.Alias
			} else if record.OauthID != "" {
				userID = record.OauthID
			}
		}

		// Company Id
		var companyID string
		if p.moesifConf.CompanyIDHeader != "" {
			companyID = fetchIDFromHeader(decodedReqBody.headers, decodedRspBody.headers, p.moesifConf.CompanyIDHeader)
		}

		// Generate random percentage
		rand.Seed(time.Now().UnixNano())
		randomPercentage := rand.Intn(100)

		// Parse sampling percentage based on user/company
		p.samplingPercentage = p.getSamplingPercentage(userID, companyID)

		if p.samplingPercentage < randomPercentage {
			log.WithFields(logrus.Fields{
				"prefix": moesifPrefix,
			}).Debug("Skipped Event due to sampling percentage: " + strconv.Itoa(p.samplingPercentage) + " and random percentage: " + strconv.Itoa(randomPercentage))
			continue
		}
		// Add Weight to the Event Model
		var eventWeight int
		if p.samplingPercentage == 0 {
			eventWeight = 1
		} else {
			eventWeight = int(math.Floor(float64(100 / p.samplingPercentage)))
		}

		event := models.EventModel{
			Request:      req,
			Response:     rsp,
			SessionToken: &record.APIKey,
			Tags:         nil,
			UserId:       &userID,
			CompanyId:    &companyID,
			Metadata:     &metadata,
			Direction:    &direction,
			Weight:       &eventWeight,
		}

		err = p.moesifAPI.QueueEvent(&event)
		if err != nil {
			log.WithFields(logrus.Fields{
				"prefix": moesifPrefix,
			}).Error("Error while writing ", data[dataIndex], err)
		}

		if p.moesifAPI.GetETag() != "" &&
			p.eTag != "" &&
			p.eTag != p.moesifAPI.GetETag() &&
			time.Now().UTC().After(p.lastUpdatedTime.Add(time.Minute*1)) {

			// Call Endpoint to fetch config
			response, err := p.moesifAPI.GetAppConfig()
			if err != nil {
				log.WithFields(logrus.Fields{
					"prefix": moesifPrefix,
				}).Debug("Error fetching application configuration with err -  " + err.Error())
				continue
			}
			p.samplingPercentage, p.eTag, p.lastUpdatedTime = p.parseConfiguration(response)
		}
	}

	return nil
}

func decodeRawData(raw string, maskHeaders []string, maskBody []string, disableCaptureBody bool) (*rawDecoded, error) {
	headersBody := strings.SplitN(raw, "\r\n\r\n", 2)

	if len(headersBody) == 0 {
		return nil, fmt.Errorf("Error while splitting raw data")
	}

	headers := decodeHeaders(headersBody[0], maskHeaders)

	var body interface{}
	if len(headersBody) == 2 && !disableCaptureBody {
		body = maskRawBody(headersBody[1], maskBody)
	}

	ret := &rawDecoded{
		headers: headers,
		body:    body,
	}

	return ret, nil
}

func decodeHeaders(headers string, maskHeaders []string) map[string]interface{} {
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

	// Mask Headers
	ret = maskData(ret, maskHeaders)

	// Transform Map to lowercase
	ret = toLowerCase(ret)

	return ret
}

func (p *MoesifPump) SetTimeout(timeout int) {
	p.timeout = timeout
}

func (p *MoesifPump) GetTimeout() int {
	return p.timeout
}
