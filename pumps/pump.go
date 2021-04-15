package pumps

import (
	"context"
	"errors"
	"fmt"

	"github.com/TykTechnologies/logrus"
	"github.com/TykTechnologies/tyk-pump/analytics"
	"github.com/kelseyhightower/envconfig"
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
}

func GetPumpByName(name string) (Pump, error) {

	if pump, ok := AvailablePumps[name]; ok && pump != nil {
		return pump, nil
	}

	return nil, errors.New(name + " Not found")
}

func processPumpEnvVars(pump Pump, log *logrus.Entry, cfg interface{}, defaultEnv string) {
	if envVar := pump.GetEnvPrefix(); envVar != "" {
		log.Debug(fmt.Sprintf("Checking %v env variables with prefix %v", pump.GetName(), envVar))
		overrideErr := envconfig.Process(envVar, cfg)
		if overrideErr != nil {
			log.Error(fmt.Sprintf("Failed to process environment variables for %v pump %v with err:%v ", envVar, pump.GetName(), overrideErr))
		}
	} else {
		log.Debug(fmt.Sprintf("Checking default %v env variables with prefix %v", pump.GetName(), defaultEnv))
		overrideErr := envconfig.Process(defaultEnv, cfg)
		if overrideErr != nil {
			log.Error(fmt.Sprintf("Failed to process environment variables for %v pump %v with err:%v ", defaultEnv, pump.GetName(), overrideErr))
		}
	}
}
