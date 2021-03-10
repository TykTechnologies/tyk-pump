package mocked

import (
	"context"

	"github.com/TykTechnologies/tyk-pump/pumps"
)

type MockedPump struct {
	CounterRequest int
	pumps.CommonPumpConfig
}

func (p *MockedPump) GetName() string {
	return "Mocked Pump"
}

func (p *MockedPump) New() pumps.Pump {
	return &MockedPump{}
}
func (p *MockedPump) Init(config interface{}) error {
	return nil
}
func (p *MockedPump) WriteData(ctx context.Context, keys []interface{}) error {
	for range keys {
		p.CounterRequest++
	}
	return nil
}