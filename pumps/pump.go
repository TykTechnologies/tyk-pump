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
	case "mongo-pump-selective":
		return AvailablePumps["mongo-pump-selective"], nil
	case "elasticsearch":
		return AvailablePumps["elasticsearch"], nil
	case "csv":
		return AvailablePumps["csv"], nil
	case "influx":
		return AvailablePumps["influx"], nil
	case "statsd":
		return AvailablePumps["statsd"], nil
	case "segment":
		return AvailablePumps["segment"], nil
	case "graylog":
		return AvailablePumps["graylog"], nil
	}

	return nil, errors.New("Not found")
}
