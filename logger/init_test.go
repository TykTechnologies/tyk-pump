package logger

import (
	"bytes"
	"github.com/sirupsen/logrus"
	"github.com/sirupsen/logrus/hooks/test"
	"testing"
)

func TestLogOutput(t *testing.T) {
	logger:= GetLogger()
	logger, hook := test.NewNullLogger()
	logger.Formatter = GetDefaultFormatter()

	logger.Error("test log error")
	assert.Equal(t, 1, len(hook.Entries))
	assert.Equal(t, logrus.ErrorLevel, hook.LastEntry().Level)
}
