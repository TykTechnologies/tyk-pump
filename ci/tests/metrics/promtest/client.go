// Package promtest is the shared toolkit for the Pump's end-to-end
// OpenTelemetry metrics suite: a tiny Prometheus HTTP API client, series and
// label matchers, a poller, and the registry of metric families the Pump
// is expected to export.
//
// It deliberately depends on the standard library only so the e2e module
// never influences the Pump's go.mod.
package promtest

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"time"
)

// Series is one Prometheus time series: its label set (including __name__)
// and the value of the latest sample.
type Series struct {
	Labels map[string]string
	Value  float64
}

// Name returns the metric name of the series.
func (s Series) Name() string { return s.Labels["__name__"] }

// Metadata describes a metric family as reported by /api/v1/metadata.
type Metadata struct {
	Type string `json:"type"`
	Help string `json:"help"`
	Unit string `json:"unit"`
}

// Client queries the Prometheus HTTP API.
type Client struct {
	BaseURL string
	HTTP    *http.Client
}

// NewClient returns a Client for the Prometheus server at baseURL.
func NewClient(baseURL string) *Client {
	return &Client{BaseURL: baseURL, HTTP: &http.Client{Timeout: 10 * time.Second}}
}

// Query runs an instant PromQL query and returns every matching series.
// An empty result is not an error.
func (c *Client) Query(ctx context.Context, query string) ([]Series, error) {
	body, err := c.get(ctx, "/api/v1/query", url.Values{"query": {query}})
	if err != nil {
		return nil, fmt.Errorf("query %q: %w", query, err)
	}
	series, err := ParseQueryResponse(body)
	if err != nil {
		return nil, fmt.Errorf("query %q: %w", query, err)
	}
	return series, nil
}

// Metadata returns the metadata Prometheus holds for the named metric family.
// ok is false when Prometheus has never scraped that family.
func (c *Client) Metadata(ctx context.Context, name string) (md Metadata, ok bool, err error) {
	body, err := c.get(ctx, "/api/v1/metadata", url.Values{"metric": {name}})
	if err != nil {
		return Metadata{}, false, fmt.Errorf("metadata %q: %w", name, err)
	}
	all, err := ParseMetadataResponse(body)
	if err != nil {
		return Metadata{}, false, fmt.Errorf("metadata %q: %w", name, err)
	}
	md, ok = all[name]
	return md, ok, nil
}

func (c *Client) get(ctx context.Context, path string, params url.Values) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.BaseURL+path+"?"+params.Encode(), nil)
	if err != nil {
		return nil, err
	}
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("prometheus returned %s: %s", resp.Status, body)
	}
	return body, nil
}

type queryResponse struct {
	Status string `json:"status"`
	Error  string `json:"error"`
	Data   struct {
		ResultType string `json:"resultType"`
		Result     []struct {
			Metric map[string]string `json:"metric"`
			Value  []json.RawMessage `json:"value"`
		} `json:"result"`
	} `json:"data"`
}

// ParseQueryResponse decodes an /api/v1/query instant-vector response.
func ParseQueryResponse(body []byte) ([]Series, error) {
	var resp queryResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("decoding response: %w", err)
	}
	if resp.Status != "success" {
		return nil, fmt.Errorf("status %q: %s", resp.Status, resp.Error)
	}
	if resp.Data.ResultType != "vector" {
		return nil, fmt.Errorf("unexpected resultType %q (want vector)", resp.Data.ResultType)
	}

	series := make([]Series, 0, len(resp.Data.Result))
	for _, r := range resp.Data.Result {
		if len(r.Value) != 2 {
			return nil, fmt.Errorf("sample for %v has %d elements (want [timestamp, value])", r.Metric, len(r.Value))
		}
		// Prometheus encodes sample values as strings ("12.5", "NaN", "+Inf").
		var raw string
		if err := json.Unmarshal(r.Value[1], &raw); err != nil {
			return nil, fmt.Errorf("sample value for %v: %w", r.Metric, err)
		}
		v, err := strconv.ParseFloat(raw, 64)
		if err != nil {
			return nil, fmt.Errorf("sample value %q for %v: %w", raw, r.Metric, err)
		}
		series = append(series, Series{Labels: r.Metric, Value: v})
	}
	return series, nil
}

type metadataResponse struct {
	Status string                `json:"status"`
	Error  string                `json:"error"`
	Data   map[string][]Metadata `json:"data"`
}

// ParseMetadataResponse decodes an /api/v1/metadata response into one
// Metadata per family (Prometheus may report several identical entries when
// more than one target exposes the family; the first is kept).
func ParseMetadataResponse(body []byte) (map[string]Metadata, error) {
	var resp metadataResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("decoding response: %w", err)
	}
	if resp.Status != "success" {
		return nil, fmt.Errorf("status %q: %s", resp.Status, resp.Error)
	}
	out := make(map[string]Metadata, len(resp.Data))
	for name, entries := range resp.Data {
		if len(entries) == 0 {
			continue
		}
		out[name] = entries[0]
	}
	return out, nil
}
