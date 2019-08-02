package pumps

import (
	"errors"
)

type Pump interface {
	GetName() string
	New() Pump
	Init(interface{}) error
	WriteData([]interface{}) error
}

func GetPumpByName(name string) (Pump, error) {

	if pump, ok := AvailablePumps[name]; ok && pump != nil {
		return pump, nil
	}

	return nil, errors.New(name + " Not found")
}
