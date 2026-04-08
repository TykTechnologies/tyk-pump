package pumps

import (
	"context"
)

type DummyPump struct {
	CommonPumpConfig
}

var dummyPrefix = "dummy-pump"
var dummyDefaultENV = PUMPS_ENV_PREFIX + "_DUMMY" + PUMPS_ENV_META_PREFIX

func (p *DummyPump) New() Pump {
	newPump := DummyPump{}
	return &newPump
}

func (p *DummyPump) GetName() string {
	return "Dummy Pump"
}

func (p *DummyPump) Init(conf interface{}) error {
	p.log = log.WithField("prefix", dummyPrefix)

	p.log.Info("Dummy Initialized")
	return nil
}

func (p *DummyPump) WriteData(ctx context.Context, data []interface{}) error {
	p.log.Info("Writing ", len(data), " records")
	return nil
}
