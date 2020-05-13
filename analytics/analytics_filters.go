package analytics

type AnalyticsFilters struct {
	OrgsIDs        []string `json:"org_ids"`
	APIIDs         []string `json:"api_ids"`
	SkippedOrgsIDs []string `json:"skip_org_ids"`
	SkippedAPIIDs  []string `json:"skip_api_ids"`
}

func (filters AnalyticsFilters) ShouldFilter(record AnalyticsRecord) bool {
	switch {
	case len(filters.SkippedAPIIDs) > 0 && stringInSlice(record.APIID, filters.SkippedAPIIDs):
		return true
	case len(filters.SkippedOrgsIDs) > 0 && stringInSlice(record.OrgID, filters.SkippedOrgsIDs):
		return true
	case len(filters.APIIDs) > 0 && !stringInSlice(record.APIID, filters.APIIDs):
		return true
	case len(filters.OrgsIDs) > 0 && !stringInSlice(record.OrgID, filters.OrgsIDs):
		return true
	}
	return false
}

func (filters AnalyticsFilters) HasFilter() bool {
	if len(filters.SkippedAPIIDs) == 0 && len(filters.SkippedOrgsIDs) == 0 && len(filters.APIIDs) == 0 && len(filters.OrgsIDs) == 0 {
		return false
	}
	return true
}

func stringInSlice(a string, list []string) bool {
	for _, b := range list {
		if b == a {
			return true
		}
	}
	return false
}
