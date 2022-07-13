package pumps

import (
	"context"

	"github.com/TykTechnologies/tyk-pump/pumps/common"
)

type DummyPump struct {
	common.Pump
}

var dummyPrefix = "dummy-pump"
var dummyDefaultENV = common.PUMPS_ENV_PREFIX + "_DUMMY" + common.PUMPS_ENV_META_PREFIX

func (p *DummyPump) GetName() string {
	return "Dummy Pump"
}

func (p *DummyPump) Init(conf interface{}) error {
	p.Log = log.WithField("prefix", dummyPrefix)

	p.Log.Info("Dummy Initialized")
	return nil
}

func (p *DummyPump) WriteData(ctx context.Context, data []interface{}) error {
	p.Log.Info("Writing ", len(data), " records")
	return nil
}
