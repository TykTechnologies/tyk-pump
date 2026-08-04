package pumps

import (
	"testing"

	"github.com/sirupsen/logrus"
	logtest "github.com/sirupsen/logrus/hooks/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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

func TestWarnOnUnresolvedKVReferences_WarnsOnReference(t *testing.T) {
	log, hook := newTestLogger()

	cfg := map[string]interface{}{"connection_string": "kv://vault/db#password"}
	warnOnUnresolvedKVReferences(log, "mongo", cfg)

	require.Len(t, hook.Entries, 1)
	assert.Equal(t, logrus.WarnLevel, hook.LastEntry().Level)
	assert.Contains(t, hook.LastEntry().Message, "mongo")
	assert.Contains(t, hook.LastEntry().Message, "config file")
}

func TestWarnOnUnresolvedKVReferences_WarnsOnInlineToken(t *testing.T) {
	log, hook := newTestLogger()

	cfg := map[string]interface{}{"url": "https://$kv{env:HOST}/v1"}
	warnOnUnresolvedKVReferences(log, "elasticsearch", cfg)

	require.Len(t, hook.Entries, 1)
	assert.Equal(t, logrus.WarnLevel, hook.LastEntry().Level)
}

func TestWarnOnUnresolvedKVReferences_SilentWithoutReferences(t *testing.T) {
	log, hook := newTestLogger()

	cfg := map[string]interface{}{"connection_string": "mongodb://localhost:27017/tyk"}
	warnOnUnresolvedKVReferences(log, "mongo", cfg)

	assert.Empty(t, hook.Entries)
}

func newTestLogger() (*logrus.Entry, *logtest.Hook) {
	l, hook := logtest.NewNullLogger()
	return logrus.NewEntry(l), hook
}
