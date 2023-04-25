package pumps

import (
	"context"
	"encoding/base64"
	"errors"
	"io/ioutil"
	"net/http"
	"net/url"
	"reflect"
	"strconv"
	"strings"

	"github.com/TykTechnologies/tyk-pump/analytics"
	"github.com/mitchellh/mapstructure"
	"github.com/resurfaceio/logger-go/v3"
)

type ResurfacePump struct {
	logger *logger.HttpLogger
	config *ResurfacePumpConfig
	CommonPumpConfig
}

type ResurfacePumpConfig struct {
	EnvPrefix string `mapstructure:"meta_env_prefix"`
	URL       string `mapstructure:"capture_url"`
	Rules     string
	Queue     []string
}

const (
	resurfacePrefix     = "resurface-pump"
	resurfacePumpName   = "Resurface Pump"
	resurfaceDefaultEnv = PUMPS_ENV_PREFIX + "_RESURFACEIO" + PUMPS_ENV_META_PREFIX
)

func (rp *ResurfacePump) New() Pump {
	newPump := ResurfacePump{}
	return &newPump
}

func (rp *ResurfacePump) GetName() string {
	return resurfacePumpName
}

func (rp *ResurfacePump) GetEnvPrefix() string {
	return rp.config.EnvPrefix
}

func (rp *ResurfacePump) Init(config interface{}) error {
	rp.config = &ResurfacePumpConfig{}
	rp.log = log.WithField("prefix", resurfacePrefix)

	err := mapstructure.Decode(config, &rp.config)
	if err != nil {
		rp.log.Debug("Failed to decode configuration: ", err)
		return err
	}

	processPumpEnvVars(rp, rp.log, rp.config, resurfaceDefaultEnv)

	opt := logger.Options{
		Rules: rp.config.Rules,
		Url:   rp.config.URL,
		Queue: rp.config.Queue,
	}
	rp.logger, err = logger.NewHttpLogger(opt)
	if err != nil {
		rp.log.Error(err)
		return err
	}
	if !rp.logger.Enabled() {
		rp.log.Info(rp.GetName() + " Initialized (Logger disabled)")
		return errors.New("logger is not enabled")
	}
	rp.log.Info(rp.GetName() + " Initialized")
	return nil
}

func parseHeaders(headersString string, existingHeaders http.Header) (headers http.Header) {
	if existingHeaders != nil {
		headers = http.Header.Clone(existingHeaders)
	} else {
		headers = http.Header{}
	}
	for _, line := range strings.Split(headersString, "\r\n") {
		header := strings.Split(line, ": ")
		if len(header) < 2 {
			continue
		}
		headers.Add(header[0], header[1])
	}
	return
}

func mapRawData(rec *analytics.AnalyticsRecord) (httpReq http.Request, httpResp http.Response, customFields map[string]string, err error) {
	var req [3]string
	var res [3]string
	tykFields := [6]string{
		"API-ID",
		"API-Key",
		"API-Name",
		"API-Version",
		"Oauth-ID",
		"Org-ID",
	}

	// Decode raw HTTP transaction from base64 strings
	rawBytesReq, err := base64.StdEncoding.DecodeString(rec.RawRequest)
	if err != nil {
		return
	}
	rawBytesRes, err := base64.StdEncoding.DecodeString(rec.RawResponse)
	if err != nil {
		return
	}
	rawReq := string(rawBytesReq)
	rawRes := string(rawBytesRes)

	// Slice first line, headers, body+trailers
	copy(req[:2], strings.SplitN(rawReq, "\r\n", 2))
	copy(res[:2], strings.SplitN(rawRes, "\r\n", 2))
	copy(req[1:], strings.SplitN(req[1], "\r\n\r\n", 2))
	copy(res[1:], strings.SplitN(res[1], "\r\n\r\n", 2))

	// Request method
	method := rec.Method
	if method == "" {
		method = strings.Fields(req[0])[0]
	}

	// Request URL
	// schema := "http" // TODO - could the AnalyticsRecord struct be modified to include the target URL Schema?
	path := rec.RawPath
	rawPath := strings.Fields(req[0])[1]
	if path == "" {
		path = rawPath
	} else if idx := strings.Index(rawPath, "?"); idx != -1 {
		path += rawPath[idx:]
	}
	if !strings.Contains(path, "://") && !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	parsedURL, err := url.Parse(path)
	if err != nil {
		return
	}

	// Request headers
	reqHeaders := parseHeaders(req[1], nil)

	// Request address
	if reqHeaders.Get("X-FORWARDED-FOR") == "" {
		reqHeaders.Add("X-FORWARDED-FOR", rec.IPAddress)
	}

	// Request host
	host := rec.Host
	if host == "" {
		host = reqHeaders.Get("Host")
	}

	// Custom Tyk fields
	customFields = make(map[string]string, len(tykFields))
	for _, field := range tykFields {
		key := strings.ReplaceAll(field, "-", "")
		if value := reflect.ValueOf(rec).Elem().FieldByName(key).String(); value != "" {
			customFields["tyk-"+field] = value
		}
	}

	// Response Status
	status := rec.ResponseCode
	if status == 0 {
		status, err = strconv.Atoi(strings.Fields(res[0])[1])
		if err != nil {
			return
		}
	}

	// Response Headers
	resHeaders := parseHeaders(res[1], nil)

	// Response Trailers
	if res[2] != "" && resHeaders.Get("Transfer-Encoding") == "chunked" && resHeaders.Get("Trailer") != "" {
		lastChunkIndex := strings.LastIndex(res[2], "0\r\n") + 3
		resHeaders = parseHeaders(res[2][lastChunkIndex:], resHeaders)
		res[2] = res[2][:lastChunkIndex]
	}

	httpReq = http.Request{
		Method: method,
		Host:   host,
		URL:    parsedURL,
		Header: reqHeaders,
		Body:   ioutil.NopCloser(strings.NewReader(req[2])),
	}

	if parsedURL.IsAbs() {
		httpReq.RequestURI = path
	}

	httpResp = http.Response{
		StatusCode: status,
		Header:     resHeaders,
		Body:       ioutil.NopCloser(strings.NewReader(res[2])),
	}

	return
}

func (rp *ResurfacePump) WriteData(ctx context.Context, data []interface{}) error {
	rp.log.Debug("Writing ", len(data), " records")

	for _, v := range data {
		decoded, ok := v.(analytics.AnalyticsRecord)
		if !ok {
			rp.log.Error("Error decoding analytic record")
			continue
		}
		if len(decoded.RawRequest) == 0 && len(decoded.RawResponse) == 0 {
			rp.log.Warn("Record dropped. Please enable Detailed Logging.")
			continue
		}

		req, resp, customFields, err := mapRawData(&decoded)
		if err != nil {
			rp.log.Error(err)
			continue
		}

		logger.SendHttpMessage(rp.logger, &resp, &req, decoded.TimeStamp.Unix()*1000, decoded.RequestTime, customFields)
	}

	rp.log.Info("Purged ", len(data), " records...")

	return nil
}
