package handler

import (
	"net/http"

	"github.com/TykTechnologies/tyk-pump/analytics"
)

func TykPumpMiddleware(next http.Handler, pumpHandler PumpHandler) http.Handler {
	return http.HandlerFunc(func(rw http.ResponseWriter, request *http.Request) {
		// TODO parse everything from the request and transform it to an analytic record
		// fake analytic record
		record := &analytics.AnalyticsRecord{}

		record.Method = "method from pump middleware"
		record.APIID = "API_ID FAKE"
		record.Path = "/middleware/tyk"
		pumpHandler.AnalyticsStorage.SendData(record)
		// Serve the HTTP Request
		next.ServeHTTP(rw, request)
	})
}
