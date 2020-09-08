package server

import (
	"fmt"
	"net/http"
	"testing"
)

func TestHealthCheck(t *testing.T) {
	go ServeHealthCheck("", 0)

	r, _ := http.NewRequest(http.MethodGet, "http://:"+fmt.Sprint(defaultHealthPort)+"/"+defaultHealthEndpoint, nil)

	resp, err := http.DefaultClient.Do(r)
	if err != nil {
		t.Fatal(err)
	}

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("Expected %d, got %d", http.StatusOK, resp.StatusCode)
	}
}
