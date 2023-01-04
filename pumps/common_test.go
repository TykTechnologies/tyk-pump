package pumps

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDecodingRequest(t *testing.T) {
	pump := &CommonPumpConfig{}
	pump.SetDecodingRequest(true)
	actualValue := pump.GetDecodedRequest()
	assert.Equal(t, actualValue, pump.decodeRequestBase64)
	assert.True(t, actualValue)
}

func TestSetDecodingResponse(t *testing.T) {
	pump := &CommonPumpConfig{}
	pump.SetDecodingResponse(true)
	actualValue := pump.GetDecodedResponse()
	assert.Equal(t, actualValue, pump.decodeResponseBase64)
	assert.True(t, actualValue)
}
