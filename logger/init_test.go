package logger

import (
	"github.com/TykTechnologies/logrus"
	"github.com/TykTechnologies/logrus/hooks/test"
	"io/ioutil"
	"os"
	"strings"
	"testing"
)

//TestFileLogOutput check if the prefix is stored in not TTY outputs
func TestFileLogOutput(t *testing.T) {

	outputFile := "test.log"
	var f *os.File
	var err error
	if f, err = os.Create(outputFile); err != nil {
		t.Errorf("create log failed:" + err.Error())
		return
	}

	logger, hook := test.NewNullLogger()
	logger.Formatter = GetDefaultFormatter()
	logger.SetOutput(f)

	logger.WithFields(logrus.Fields{
		"prefix": "test-prefix",
	}).Errorf("test error log")

	err = f.Sync()
	if err != nil {
		t.Error("Sync test logs file:" + err.Error())
	}
	err = f.Close()
	if err != nil {
		t.Error("Closing test logs file:" + err.Error())
	}

	//check the logs in the hook
	if len(hook.Entries) != 1 {
		t.Error("logger doesn't contain the correct number of records")
	}

	//Now check the content in the file
	b, err := ioutil.ReadFile(outputFile)
	if err != nil {
		t.Error("Reading test logs file")
	}
	fileContent := string(b)
	if !strings.Contains(fileContent, "prefix") {
		t.Error("Prefix is not being added to logs information")
	}

	err = os.Remove(outputFile)
	if err != nil {
		t.Error("Error removing test logs file:" + err.Error())
	}
}
