package pumps

import (
	"context"
)

type DummyPump struct {
	CommonPumpConfig
}

var dummyPrefix = "dummy-pump"
var dummyDefaultENV = PUMPS_ENV_PREFIX + "_DUMMY" + PUMPS_ENV_META_PREFIX

// reqproof:implements SW-REQ-026
func (p *DummyPump) New() Pump {
	newPump := DummyPump{}
	return &newPump
}

// reqproof:implements SW-REQ-026
func (p *DummyPump) GetName() string {
	return "Dummy Pump"
}

// reqproof:implements SW-REQ-026
func (p *DummyPump) Init(conf interface{}) error {
	p.log = log.WithField("prefix", dummyPrefix)

	p.log.Info("Dummy Initialized")
	return nil
}

// reqproof:implements SW-REQ-026
func (p *DummyPump) WriteData(ctx context.Context, data []interface{}) error {
	p.log.Info("Writing ", len(data), " records")
	return nil
}
