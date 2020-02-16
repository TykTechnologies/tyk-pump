package pumps

import (
	"context"
	"errors"
)

type Pump interface {
	GetName() string
	New() Pump
	Init(interface{}) error
	WriteData(context.Context, []interface{}) error
	SetTimeout(timeout int)
	GetTimeout() int
}

func GetPumpByName(name string) (Pump, error) {

	if pump, ok := AvailablePumps[name]; ok && pump != nil {
		return pump, nil
	}

	return nil, errors.New(name + " Not found")
}
