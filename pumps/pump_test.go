package pumps

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGetPumpByName(t *testing.T) {

	dummyType, err := GetPumpByName("dummy")
	assert.NoError(t, err)
	assert.Equal(t, dummyType, &DummyPump{})

	invalidPump, err := GetPumpByName("xyz")
	assert.Error(t, err)
	assert.Nil(t, invalidPump)

	mongoPump, err := GetPumpByName("MONGO")
	assert.NoError(t, err)
	assert.Equal(t, mongoPump, &MongoPump{})

	sqlPump, err := GetPumpByName("SqL")
	assert.NoError(t, err)
	assert.Equal(t, sqlPump, &SQLPump{})
}
