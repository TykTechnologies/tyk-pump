package ors

import (
	"container/list"
	"encoding/base64"
	"github.com/TykTechnologies/tyk-pump/analytics"
)

func contentListToRawRequest(processedRequestContent list.List) string {
	return ""
}

func processRequestContent(requestContentAsList list.List) list.List {
	return list.List{}
}

func getRequestElements(stringRequest string) list.List {
	return list.List{}
}

func getRequestContent(decodedRequestContent string) list.List {
	elements := getRequestElements(decodedRequestContent)
	return elements
}

func CleanDecodedRawRequest(decodedRawRequest []byte) []byte {
	println("hello")
	decodedRawRequest = decodedRawRequest
	decodedRawRequestString := string(decodedRawRequest)
	requestContentAsList := getRequestContent(decodedRawRequestString)
	processedRequestContent := processRequestContent(requestContentAsList)
	// TODO Check if the return should be in json or as a request with the manipulated Request
	processedRawRequest := contentListToRawRequest(processedRequestContent)
	return []byte(processedRawRequest)
}

func ProcessRawRequest(rawEncodedRequest string) string {
	decodedRawReq, _ := base64.StdEncoding.DecodeString(rawEncodedRequest)
	cleanedDecodedRawReq := CleanDecodedRawRequest(decodedRawReq)
	cleanedEncodedRawReq := base64.StdEncoding.EncodeToString(cleanedDecodedRawReq)
	return cleanedEncodedRawReq
}

func CleanAnalyticsRecord(analyticsRecord analytics.AnalyticsRecord) analytics.AnalyticsRecord {
	analyticsRecord = analyticsRecord
	rawRequest := analyticsRecord.RawRequest
	cleanedEncodedRawRequest := ProcessRawRequest(rawRequest)
	analyticsRecord.RawRequest = cleanedEncodedRawRequest
	return analyticsRecord
}
