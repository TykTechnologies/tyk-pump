package pumps

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Verifies: SW-REQ-017
// SW-REQ-017:nominal:nominal
// Verifies: INT-REQ-004
// MCDC INT-REQ-004: contract_honoured=F, pump_methods_called=F => TRUE
// MCDC INT-REQ-004: contract_honoured=F, pump_methods_called=T => FALSE
// MCDC INT-REQ-004: contract_honoured=T, pump_methods_called=T => TRUE
//
// GetPumpByName is the contract entry-point (pump_methods_called=T). For each pump_method
// invocation the assertion that the returned interface is the expected concrete pump type
// (or that an error is surfaced for unknown names) proves contract_honoured=T -> TRUE row.
// A regression where the registry returned an incorrect pump for a name would land on the
// FALSE row (caught by assert.Equal). The vacuous TRUE arm is "no invocation".
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

// Verifies: SW-REQ-017
// SW-REQ-017:support_matrix_enforced:nominal
func TestGetPumpByName_SQSSupported(t *testing.T) {
	p, err := GetPumpByName("SQS")
	require.NoError(t, err)
	require.IsType(t, &SQSPump{}, p)
	require.IsType(t, &SQSPump{}, p.New())
}

// Verifies: SW-REQ-017
// SW-REQ-017:support_matrix_enforced:nominal
func TestGetPumpByName_KinesisSupported(t *testing.T) {
	p, err := GetPumpByName("KINESIS")
	require.NoError(t, err)
	require.IsType(t, &KinesisPump{}, p)
	require.IsType(t, &KinesisPump{}, p.New())
}
