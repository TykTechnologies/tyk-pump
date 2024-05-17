package pumps

import (
	"github.com/TykTechnologies/tyk-pump/analytics"
	"github.com/sirupsen/logrus"
)

type CommonPumpConfig struct {
	filters               analytics.AnalyticsFilters
	timeout               int
	maxRecordSize         int
	OmitDetailedRecording bool
	log                   *logrus.Entry
	ignoreFields          []string
	decodeResponseBase64  bool
	decodeRequestBase64   bool
}

func (p *CommonPumpConfig) SetFilters(filters analytics.AnalyticsFilters) {
	p.filters = filters
}
func (p *CommonPumpConfig) GetFilters() analytics.AnalyticsFilters {
	return p.filters
}
func (p *CommonPumpConfig) SetTimeout(timeout int) {
	p.timeout = timeout
}

func (p *CommonPumpConfig) GetTimeout() int {
	return p.timeout
}

func (p *CommonPumpConfig) SetOmitDetailedRecording(OmitDetailedRecording bool) {
	p.OmitDetailedRecording = OmitDetailedRecording
}
func (p *CommonPumpConfig) GetOmitDetailedRecording() bool {
	return p.OmitDetailedRecording
}

func (p *CommonPumpConfig) GetEnvPrefix() string {
	return ""
}

func (p *CommonPumpConfig) Shutdown() error {
	return nil
}

func (p *CommonPumpConfig) SetMaxRecordSize(size int) {
	p.maxRecordSize = size
}

func (p *CommonPumpConfig) GetMaxRecordSize() int {
	return p.maxRecordSize
}

func (p *CommonPumpConfig) SetLogLevel(level logrus.Level) {
	p.log.Level = level
}

func (p *CommonPumpConfig) SetIgnoreFields(fields []string) {
	p.ignoreFields = fields
}

func (p *CommonPumpConfig) GetIgnoreFields() []string {
	return p.ignoreFields
}

func (p *CommonPumpConfig) SetDecodingResponse(decoding bool) {
	p.decodeResponseBase64 = decoding
}

func (p *CommonPumpConfig) SetDecodingRequest(decoding bool) {
	p.decodeRequestBase64 = decoding
}

func (p *CommonPumpConfig) GetDecodedRequest() bool {
	return p.decodeRequestBase64
}

func (p *CommonPumpConfig) GetDecodedResponse() bool {
	return p.decodeResponseBase64
}
