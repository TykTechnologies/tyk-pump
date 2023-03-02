package pumps

import (
	"testing"
)

func TestGetPumpByName(t *testing.T) {
	name := "dummy"
	pmpType, err := GetPumpByName(name)

	if err != nil || pmpType == nil {
		t.Fail()
	}

	name2 := "xyz"
	pmpType2, err2 := GetPumpByName(name2)

	if err2 == nil || pmpType2 != nil {
		t.Fail()
	}
}
