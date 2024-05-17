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

func GetPumpByName(name string) (Pump, error) {

	if pump, ok := AvailablePumps[strings.ToLower(name)]; ok && pump != nil {
		return pump, nil
	}

	return nil, errors.New(name + " Not found")
}

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
