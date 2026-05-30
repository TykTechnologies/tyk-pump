package analytics

import "testing"

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
