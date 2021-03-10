package helpers

import (
	"net/http"

	"github.com/TykTechnologies/tyk-pump/analytics"
	"github.com/TykTechnologies/tyk-pump/handler"
)

func TykPumpMiddleware(next http.Handler, pumpHandler handler.PumpHandler) http.Handler {
	return http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
		// TODO parse everything from the request and transform it to an analytic record
		// fake analytic record
		record := &analytics.AnalyticsRecord{}

		record.Method = r.Method
		record.APIID = r.Header.Get("api_id")
		record.Path = r.URL.Path
		record.RawPath = r.URL.RawPath
		record.APIVersion = r.Header.Get("version")
		//TODO Finish this :P


		pumpHandler.AnalyticsStorage.SendData(record)
		// Serve the HTTP Request
		next.ServeHTTP(rw, r)
	})
}

