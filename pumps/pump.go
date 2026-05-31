package pumps

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/TykTechnologies/tyk-pump/analytics"
	"github.com/kelseyhightower/envconfig"
	"github.com/sirupsen/logrus"
)

const PUMPS_ENV_PREFIX = "TYK_PMP_PUMPS"
const PUMPS_ENV_META_PREFIX = "_META"

type Pump interface {
	GetName() string
	New() Pump
	Init(interface{}) error
	WriteData(context.Context, []interface{}) error
	SetFilters(analytics.AnalyticsFilters)
	GetFilters() analytics.AnalyticsFilters
	SetTimeout(timeout int)
	GetTimeout() int
	SetOmitDetailedRecording(bool)
	GetOmitDetailedRecording() bool
	GetEnvPrefix() string
	Shutdown() error
	SetMaxRecordSize(size int)
	GetMaxRecordSize() int
	SetLogLevel(logrus.Level)
	SetIgnoreFields([]string)
	GetIgnoreFields() []string
	SetDecodingResponse(bool)
	GetDecodedResponse() bool
	SetDecodingRequest(bool)
	GetDecodedRequest() bool
}

type UptimePump interface {
	GetName() string
	Init(interface{}) error
	WriteUptimeData(data []interface{})
}

// reqproof:implements SW-REQ-017
func GetPumpByName(name string) (Pump, error) {

	if pump, ok := AvailablePumps[strings.ToLower(name)]; ok && pump != nil { //mcdc:ignore the ok && pump != nil short-circuit independent-effect proof requires three input rows: (ok=T && pump!=nil=T), (ok=F, pump=skipped), and (ok=T && pump=nil). The third row is structurally unreachable — AvailablePumps is populated by init() with non-nil pump pointers and never has a nil-value entry. Driving pump==nil with ok==T requires mutating the package-level map mid-test which would race with other tests. KI mcdc-pumps-below-95.
		return pump, nil
	}

	return nil, errors.New(name + " Not found")
}

// reqproof:implements SW-REQ-017
func processPumpEnvVars(pump Pump, log *logrus.Entry, cfg interface{}, defaultEnv string) {
	if envVar := pump.GetEnvPrefix(); envVar != "" {
		log.Debug(fmt.Sprintf("Checking %s env variables with prefix %s", pump.GetName(), envVar))
		overrideErr := envconfig.Process(envVar, cfg)
		if overrideErr != nil {
			log.Error(fmt.Sprintf("Failed to process environment variables for %s pump %s with err:%v ", envVar, pump.GetName(), overrideErr))
		}
	} else {
		log.Debug(fmt.Sprintf("Checking default %s env variables with prefix %s", pump.GetName(), defaultEnv))
		overrideErr := envconfig.Process(defaultEnv, cfg)
		if overrideErr != nil {
			log.Error(fmt.Sprintf("Failed to process environment variables for %s pump %s with err:%v ", defaultEnv, pump.GetName(), overrideErr))
		}
	}
}
