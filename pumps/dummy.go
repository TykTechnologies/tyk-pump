package pumps

import (
	"github.com/Sirupsen/logrus"
)

type DummyPump struct{}

var dummyPrefix string = "dummy-pump"

func (p *DummyPump) New() Pump {
	newPump := DummyPump{}
	return &newPump
}

func (p *DummyPump) GetName() string {
	return "Dummy Pump"
}

func (p *DummyPump) Init(conf interface{}) error {
	log.WithFields(logrus.Fields{
		"prefix": dummyPrefix,
	}).Debug("Dummy Initialized")
	return nil
}

func (p *DummyPump) WriteData(data []interface{}) error {
	log.WithFields(logrus.Fields{
		"prefix": dummyPrefix,
	}).Info("Writing ", len(data), " records")
	return nil
}
