package analytics

import (
	"testing"

	"github.com/oschwald/maxminddb-golang"
)

// Verifies: SW-REQ-071
// Verifies: SYS-REQ-034
// SW-REQ-071:nominal:negative
// SW-REQ-071:error_handling:negative
// SYS-REQ-034:nominal:negative
func TestGeoIPLookup_Coverable(t *testing.T) {
	// Empty IP returns (nil, nil) before touching the DB.
	geo, err := GeoIPLookup("", nil)
	if geo != nil || err != nil {
		t.Fatalf("empty IP: expected (nil,nil), got (%v,%v)", geo, err)
	}
	// A syntactically invalid IP fails before any DB lookup.
	geo, err = GeoIPLookup("not-an-ip", nil)
	if geo != nil || err == nil {
		t.Fatalf("invalid IP: expected (nil, error), got (%v,%v)", geo, err)
	}
}

// Verifies: SW-REQ-071
// Verifies: SYS-REQ-034
// SW-REQ-071:nominal:negative
// SYS-REQ-034:nominal:negative
func TestGetGeo_NilDatabase(t *testing.T) {
	a := &AnalyticsRecord{}
	a.GetGeo("1.2.3.4", nil) // nil DB path must short-circuit without panic
	if a.Geo.Country.ISOCode != "" {
		t.Fatal("nil GeoIPDB must not populate geo fields")
	}
}

// Verifies: SW-REQ-071
// openSampleGeoIPDB opens analytics/testdata/sample.mmdb, a tiny synthetic
// MaxMind City database produced by mmdbwriter. It contains a single network
// 1.2.3.0/24 with iso_code=ZZ / city=Sample City / lat=1 / lon=2 / tz=UTC.
// The fixture is read-only and shared by GeoIP tests that need a real
// *maxminddb.Reader to drive the GeoIPDB == nil F-side and the ip == nil
// F-side in GeoIPLookup (analytics.go:374 and :404).
func openSampleGeoIPDB(t *testing.T) *maxminddb.Reader {
	t.Helper()
	db, err := maxminddb.Open("testdata/sample.mmdb")
	if err != nil {
		t.Fatalf("open sample.mmdb fixture: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return db
}

// Verifies: SW-REQ-071
// Verifies: SYS-REQ-034
// SW-REQ-071:nominal:positive
// SYS-REQ-034:nominal:positive
// Drives the F-side of `GeoIPDB == nil` in GetGeo (analytics.go:374) by
// providing a real *maxminddb.Reader and a valid IP that the synthetic
// fixture maps to a known city record. Together with the existing
// TestGetGeo_NilDatabase, this closes the MC/DC pair for the nil-DB guard.
func TestGetGeo_RealDatabase_PopulatesGeo(t *testing.T) {
	db := openSampleGeoIPDB(t)

	a := &AnalyticsRecord{}
	a.GetGeo("1.2.3.4", db)

	if got := a.Geo.Country.ISOCode; got != "ZZ" {
		t.Fatalf("ISOCode = %q, want %q", got, "ZZ")
	}
	if got := a.Geo.City.Names["en"]; got != "Sample City" {
		t.Fatalf("City.Names[en] = %q, want %q", got, "Sample City")
	}
	if got := a.Geo.Location.TimeZone; got != "UTC" {
		t.Fatalf("Location.TimeZone = %q, want %q", got, "UTC")
	}
}

// Verifies: SW-REQ-071
// Verifies: SYS-REQ-034
// SW-REQ-071:error_handling:positive
// SYS-REQ-034:error_handling:positive
// Drives the F-side of `ip == nil` in GeoIPLookup (analytics.go:404) by
// providing a real *maxminddb.Reader together with a syntactically valid
// IP that net.ParseIP can resolve. Pairs with the existing invalid-IP case
// in TestGeoIPLookup_Coverable to close the MC/DC pair for the ip==nil
// guard, and additionally exercises the F-side of `err != nil` on the
// Lookup call by using a fixture-known network.
func TestGeoIPLookup_RealDatabase_ParsedIP(t *testing.T) {
	db := openSampleGeoIPDB(t)

	geo, err := GeoIPLookup("1.2.3.4", db)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if geo == nil {
		t.Fatal("expected non-nil GeoData for IP within fixture network")
	}
	if geo.Country.ISOCode != "ZZ" {
		t.Fatalf("Country.ISOCode = %q, want %q", geo.Country.ISOCode, "ZZ")
	}

	// A second valid IP outside the fixture network still parses successfully
	// (drives the F-side of ip == nil) and Lookup returns a zero-value record
	// without error, exercising the same `err != nil` F-side path.
	geo, err = GeoIPLookup("8.8.8.8", db)
	if err != nil {
		t.Fatalf("unexpected error for out-of-network IP: %v", err)
	}
	if geo == nil {
		t.Fatal("expected non-nil GeoData even for out-of-network IP")
	}
}
