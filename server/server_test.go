package server

import (
	"fmt"
	"net/http"
	"testing"
)

func TestHealthCheck(t *testing.T) {
	go ServeHealthCheck("", 9090)

	r, _ := http.NewRequest(http.MethodGet, "http://:"+fmt.Sprint(9090)+"/"+defaultHealthEndpoint, nil)

	var err error
	var resp *http.Response
	for i := 0; i < 4; i++ {
		resp, err = http.DefaultClient.Do(r)
		if err == nil {
			break
		}
	}
	if err != nil {
		t.Fatal(err)
	}

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("Expected %d, got %d", http.StatusOK, resp.StatusCode)
	}
}
