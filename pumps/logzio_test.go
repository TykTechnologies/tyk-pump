package pumps

import (
	"testing"

	"github.com/mitchellh/mapstructure"
)

func TestLogzioInit(t *testing.T) {
	conf := NewLogzioPumpConfig()

	_, err := NewLogzioClient(conf)
	if err == nil {
		t.Fatal("Default configuration should return an error - empty token")
	}

	conf.Token = testToken
	conf.DrainDuration = "3"
	_, err = NewLogzioClient(conf)
	if err == nil {
		t.Fatal("Valid time units for drain duration are 'ns', 'us' (or 'µs')," +
			" 'ms', 's', 'm', 'h'")
	}

	conf.DrainDuration = "3s"
	conf.Token = testToken
	conf.DiskThreshold = -123
	_, err = NewLogzioClient(conf)
	if err == nil {
		t.Fatal("Valid disk threshold should be between 0 and 100. Not -123")
	}

	conf.DiskThreshold = maxDiskThreshold + 1
	_, err = NewLogzioClient(conf)
	if err == nil {
		t.Fatal("Valid disk threshold should be between 0 and 100. Not 101")
	}
}

func TestLogzioDecodeWithDefaults(t *testing.T) {
	config := map[string]interface{}{
		"token": "123456789",
	}
	pconfig := NewLogzioPumpConfig()
	err := mapstructure.Decode(config, pconfig)
	if err != nil {
		t.Fatal("Failed to decode valid configuration")
	}

	if pconfig.Token != config["token"] {
		t.Fatal("Failed to decode token field")
	}
}

func TestLogzioDecodeOverrideDefaults(t *testing.T) {
	config := map[string]interface{}{
		"token":            "123456789",
		"check_disk_space": false,
		"disk_threshold":   10,
		"drain_duration":   "4s",
		"queue_dir":        "./my-logzio-queue",
		"url":              "http://localhost:8088/",
	}
	pconfig := NewLogzioPumpConfig()
	err := mapstructure.Decode(config, pconfig)
	if err != nil {
		t.Fatal("Failed to decode valid configuration")
	}

	if pconfig.Token != config["token"] {
		t.Fatal("Failed to decode token field")
	}

	if pconfig.CheckDiskSpace != config["check_disk_space"] ||
		pconfig.DiskThreshold != config["disk_threshold"] ||
		pconfig.DrainDuration != config["drain_duration"] ||
		pconfig.QueueDir != config["queue_dir"] ||
		pconfig.URL != config["url"] {
		t.Fatalf("Failed to override one of the default configurations: %+v", pconfig)
	}
}
