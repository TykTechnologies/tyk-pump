package analytics

import (
	b64 "encoding/base64"
	"strings"
	"testing"
)

func TestNew(t *testing.T) {
	a := AnalyticsRecordAggregate{}
	newA := a.New()
	if newA.APIID == nil && newA.ApiEndpoint == nil {
		t.Fatal("New should have initialised APIID and ApiEndpoint")
	}
}

func TestDoHash(t *testing.T) {
	res := doHash("test")
	resDecoded, _ := b64.StdEncoding.DecodeString(res + "==")
	if string(resDecoded) != "test" {
		t.Fatal("Decoded Hash should be 'test'.")
	}
}

func TestIgnoreTag(t *testing.T) {
	tag := "key-test"
	prefixList := []string{"t-"}
	res := ignoreTag(tag, prefixList)

	if res == false {
		t.Fatal("ignoreTag should be true when tag has a key- prefix")
	}

	tag = "t-test"
	res = ignoreTag(tag, prefixList)
	if res == false {
		t.Fatal("ignoreTag should be true when tag has a prefix in the prefixList")
	}

	tag = "nest"
	res = ignoreTag(tag, prefixList)
	if res == true {
		t.Fatal("ignoreTag should be false when it has no tags to ignore.s")
	}
}

func TestReplaceUnsupportedChars(t *testing.T) {
	path := "/test.no"
	res := replaceUnsupportedChars(path)
	if strings.Contains(res, ".") {
		t.Fatal("replaceUnsupportedChars should replace the dots.")
	}
}
