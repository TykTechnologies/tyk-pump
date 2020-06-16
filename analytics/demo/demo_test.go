package demo

import (
	"github.com/TykTechnologies/tyk-pump/analytics"
	"github.com/gocraft/health"
	"testing"
	"time"
)

func TestDemoInit(t *testing.T) {
	DemoInit("org", "api", "")
	if apiID != "api" {
		t.Fatal("apiID should be api.")
	}
	if apiVersion != "Default" {
		t.Fatal("apiVersion should be Default.")
	}
	if len(apiKeys) == 0 {
		t.Fatal("apiKeys len shouldn't be 0.")

	}
}

func TestGenerateDemoData(t *testing.T) {
	DemoInit("org", "api", "")
	totalWrites := 0
	exampleElement := analytics.AnalyticsRecord{}
	writerFn := func(elements []interface{}, job *health.Job, start time.Time, delay int) {
		for i, elem := range elements {
			if i == 0 {
				exampleElement = elem.(analytics.AnalyticsRecord)
			}
			totalWrites++
		}
	}

	GenerateDemoData(time.Now(), 1, "org", writerFn)

	if totalWrites == 0 {
		t.Fatal("GenerateDemoData should have generated data.")
	}

	if exampleElement.APIID == "" || exampleElement.OrgID != "org" {
		t.Fatal("GenerateDemoData create a malformed analytic record.")
	}

}
