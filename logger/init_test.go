package logger

import (
	"io/ioutil"
	"os"
	"strings"
	"testing"

	"github.com/TykTechnologies/logrus"
)

//TestFormatterWithForcedPrefixFileOutput check if the prefix is stored in not TTY outputs
func TestFormatterWithForcedPrefixFileOutput(t *testing.T) {

	outputFile := "test.log"
	var f *os.File
	var err error
	if f, err = os.Create(outputFile); err != nil {
		t.Errorf("create log test file failed")
		return
	}

	logger := log
	logger.Out = f

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
