package analytics

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCode_ProcessStatusCodes(t *testing.T) {
	errorMap := map[string]int{
		"400": 4,
		"481": 3, // not existing error code
		"482": 2, // not existing error code
		"666": 3, // invalid code
	}

	c := Code{}
	c.ProcessStatusCodes(errorMap)

	assert.Equal(t, 4, c.Code400)
	assert.Equal(t, 5, c.Code4x)

}
