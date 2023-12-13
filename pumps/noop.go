package pumps

import (
	"context"
)

type NoopPump struct {
	CommonPumpConfig
}

var noopPrefix = "noop-pump"

func (p *NoopPump) New() Pump {
	return &NoopPump{}
}

func (p *NoopPump) GetName() string {
	return "Noop Pump"
}

func (p *NoopPump) GetEnvPrefix() string {
	return "noop"
}

func (p *NoopPump) Init(conf interface{}) error {
	p.log = log.WithField("prefix", p.GetEnvPrefix())
	p.log.Info(p.GetName() + " Initialized")
	return nil
}

func (p *NoopPump) WriteData(ctx context.Context, data []interface{}) error {
	p.log.Debug("Attempting to purge ", len(data), " records...")
	p.log.Info("Purged ", len(data), " records...")
	return nil
}
