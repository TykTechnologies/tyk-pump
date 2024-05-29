package analytics

type AnalyticsFilters struct {
	// Filters pump data by an allow list of org_ids.
	OrgsIDs []string `json:"org_ids"`
	// Filters pump data by an allow list of api_ids.
	APIIDs []string `json:"api_ids"`
	// Filters pump data by an allow list of response_codes.
	ResponseCodes []int `json:"response_codes"`
	// Filters pump data by a block list of org_ids.
	SkippedOrgsIDs []string `json:"skip_org_ids"`
	// Filters pump data by a block list of api_ids.
	SkippedAPIIDs []string `json:"skip_api_ids"`
	// Filters pump data by a block list of response_codes.
	SkippedResponseCodes []int `json:"skip_response_codes"`
}

func (filters AnalyticsFilters) ShouldFilter(record AnalyticsRecord) bool {
	switch {
	case len(filters.SkippedAPIIDs) > 0 && stringInSlice(record.APIID, filters.SkippedAPIIDs):
		return true
	case len(filters.SkippedOrgsIDs) > 0 && stringInSlice(record.OrgID, filters.SkippedOrgsIDs):
		return true
	case len(filters.SkippedResponseCodes) > 0 && intInSlice(record.ResponseCode, filters.SkippedResponseCodes):
		return true
	case len(filters.APIIDs) > 0 && !stringInSlice(record.APIID, filters.APIIDs):
		return true
	case len(filters.OrgsIDs) > 0 && !stringInSlice(record.OrgID, filters.OrgsIDs):
		return true
	case len(filters.ResponseCodes) > 0 && !intInSlice(record.ResponseCode, filters.ResponseCodes):
		return true
	}
	return false
}

func (filters AnalyticsFilters) HasFilter() bool {
	if len(filters.SkippedAPIIDs) == 0 && len(filters.SkippedOrgsIDs) == 0 && len(filters.ResponseCodes) == 0 && len(filters.APIIDs) == 0 && len(filters.OrgsIDs) == 0 && len(filters.SkippedResponseCodes) == 0 {
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

func intInSlice(a int, list []int) bool {
	for _, b := range list {
		if b == a {
			return true
		}
	}
	return false
}
