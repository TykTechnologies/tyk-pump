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
	switch name {
	case "dummy":
		return AvailablePumps["dummy"], nil
	case "mongo":
		return AvailablePumps["mongo"], nil
	case "elasticsearch":
		return AvailablePumps["elasticsearch"], nil
	case "csv":
		return AvailablePumps["csv"], nil
	case "graylog":
		return AvailablePumps["graylog"], nil
	}

	return nil, errors.New("Not found")
}
