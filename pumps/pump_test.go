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

func TestHasUnresolvedKVReference(t *testing.T) {
	cases := []struct {
		name string
		cfg  any
		want bool
	}{
		{"whole-value reference", map[string]any{"connection_string": "kv://vault/db#password"}, true},
		{"inline token", map[string]any{"url": "https://$kv{env:HOST}/v1"}, true},
		{"reference nested in map", map[string]any{"meta": map[string]any{"dsn": "kv://secrets/dsn"}}, true},
		{"malformed marker still detected", map[string]any{"x": "$kv{env:X"}, true},
		{"plain value, no reference", map[string]any{"connection_string": "mongodb://localhost:27017/tyk"}, false},
		{"empty config", map[string]any{}, false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, hasUnresolvedKVReference(tc.cfg))
		})
	}
}
