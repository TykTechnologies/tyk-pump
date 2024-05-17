package demo

import (
	"math/rand"
	"strings"
	"time"

	"github.com/TykTechnologies/tyk-pump/analytics"
	"github.com/TykTechnologies/tyk-pump/logger"

	"github.com/gocraft/health"
	"github.com/gofrs/uuid"
)

var (
	apiKeys    []string
	apiID      string
	apiVersion string
	log        = logger.GetLogger()
)

func DemoInit(orgId, apiId, version string) {
	apiID = apiId
	GenerateAPIKeys(orgId)
	apiVersion = version
	if version == "" {
		apiVersion = "Default"
	}
}

func randomInRange(min, max int) int {
	rand.Seed(time.Now().UnixNano())
	return rand.Intn(max-min) + min
}

func randomMethod() string {
	methods := []string{"GET", "PUT", "POST", "DELETE", "OPTIONS", "HEAD"}

	rand.Seed(time.Now().Unix())
	return methods[rand.Intn(len(methods))]
}

func randomPath() string {
	seedSet := []string{"/widget", "/foo", "/beep", "/boop"}
	wordset := []string{
		"unconvergent",
		"choragic",
		"umbellate",
		"redischarging",
		"quebrada",
		"contextured",
		"prerequest",
		"neckless",
		"billhook",
		"cobaltammine",
		"diaphototropism",
		"paraiba",
		"unsesquipedalian",
		"labyrinth",
		"interesterification",
		"dahlonega",
		"countryfiedness",
		"cayuga",
		"kernelled",
		"unprophesied",
	}

	depth := randomInRange(1, 3)
	path := seedSet[rand.Intn(len(seedSet))]
	for i := 1; i <= depth; i++ {
		path += "/" + wordset[rand.Intn(len(wordset))]
	}

	return path
}

func randomAPI() (string, string) {
	if apiID != "" {
		return "Foo Bar", apiID
	}
	names := [][]string{
		{"Foo Bar Baz API", "de6e4d9ddde34d1657a6d93fab835abd"},
		{"Wibble Wobble API", "de6e4d9ddde34d1657a6d92fab935aba"},
		{"Wonky Ponky API", "de6e4d9ddde34d1657a6d91fab836abb"},
	}

	api := names[rand.Intn(len(names))]
	return api[0], api[1]
}

func getUA() string {
	userAgents := []string{
		"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/56.0.2924.87 Safari/537.36",
		"Mozilla/5.0 (Windows NT 6.1; WOW64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/56.0.2924.87 Safari/537.36",
		"Mozilla/5.0 (Windows NT 10.0; WOW64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/56.0.2924.87 Safari/537.36",
		"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_12_3) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/56.0.2924.87 Safari/537.36",
		"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_12_3) AppleWebKit/602.4.8 (KHTML, like Gecko) Version/10.0.3 Safari/602.4.8",
		"Mozilla/5.0 (Windows NT 6.1; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/56.0.2924.87 Safari/537.36",
		"Mozilla/5.0 (Windows NT 10.0; WOW64; rv:51.0) Gecko/20100101 Firefox/51.0",
		"Mozilla/5.0 (Windows NT 10.0; WOW64; Trident/7.0; rv:11.0) like Gecko",
	}

	return userAgents[rand.Intn(len(userAgents))]
}

func responseCode() int {
	codes := []int{
		200,
		200,
		200,
		403,
		200,
		500,
		200,
		200,
		200,
		200,
	}

	return codes[rand.Intn(len(codes))]
}

func GenerateAPIKeys(orgId string) {
	set := make([]string, 50)
	for i := 0; i < len(set); i++ {
		set[i] = generateAPIKey(orgId)
	}
	apiKeys = set
}

func generateAPIKey(orgId string) string {
	u1, err := uuid.NewV4()
	if err != nil {
		log.WithError(err).Error("failed to generate UUID")
	}
	id := strings.Replace(u1.String(), "-", "", -1)
	return orgId + id
}

func getRandomKey(orgId string) string {
	if len(apiKeys) == 0 {
		GenerateAPIKeys(orgId)
	}
	return apiKeys[rand.Intn(len(apiKeys))]
}

func country() string {
	codes := []string{
		"RU",
		"US",
		"UK",
	}
	return codes[rand.Intn(len(codes))]
}

func GenerateDemoData(days, recordsPerHour int, orgID string, demoFutureData, trackPath bool, writer func([]interface{}, *health.Job, time.Time, int)) {
	t := time.Now()
	start := time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC)
	count := 0
	// If we are generating future data, we want to start at the current date and create data for the next X days
	if demoFutureData {
		for d := 0; d < days; d++ {
			for h := 0; h < 24; h++ {
				WriteDemoData(start, d, h, recordsPerHour, orgID, trackPath, writer)
			}
			count++
			log.Infof("Finished %d of %d\n", count, days)
		}
		return
	}

	// Otherwise, we want to start at the (current date - X days) and create data until yesterday's date
	for d := days; d > 0; d-- {
		for h := 0; h < 24; h++ {
			WriteDemoData(start, -d, h, recordsPerHour, orgID, trackPath, writer)
		}
		count++
		log.Infof("Finished %d of %d\n", count, days)
	}
}

func WriteDemoData(start time.Time, d, h, recordsPerHour int, orgID string, trackPath bool, writer func([]interface{}, *health.Job, time.Time, int)) {
	set := []interface{}{}
	ts := start.AddDate(0, 0, d)
	ts = ts.Add(time.Duration(h) * time.Hour)
	// Generate daily entries
	var volume int
	if recordsPerHour > 0 {
		volume = recordsPerHour
	} else {
		volume = randomInRange(300, 500)
	}
	timeDifference := 3600 / volume // this is the difference in seconds between each record
	nextTimestamp := ts             // this is the timestamp of the next record
	for i := 0; i < volume; i++ {
		r := GenerateRandomAnalyticRecord(orgID, trackPath)
		r.Day = nextTimestamp.Day()
		r.Month = nextTimestamp.Month()
		r.Year = nextTimestamp.Year()
		r.Hour = nextTimestamp.Hour()
		r.TimeStamp = nextTimestamp
		nextTimestamp = nextTimestamp.Add(time.Second * time.Duration(timeDifference))

		set = append(set, r)
	}

	writer(set, nil, time.Now(), 10)
}

func GenerateRandomAnalyticRecord(orgID string, trackPath bool) analytics.AnalyticsRecord {
	p := randomPath()
	api, apiID := randomAPI()
	ts := time.Now()
	r := analytics.AnalyticsRecord{
		Method:        randomMethod(),
		Path:          p,
		RawPath:       p,
		ContentLength: int64(randomInRange(0, 999)),
		UserAgent:     getUA(),
		Day:           ts.Day(),
		Month:         ts.Month(),
		Year:          ts.Year(),
		Hour:          ts.Hour(),
		ResponseCode:  responseCode(),
		APIKey:        getRandomKey(orgID),
		TimeStamp:     ts,
		APIVersion:    apiVersion,
		APIName:       api,
		APIID:         apiID,
		OrgID:         orgID,
		OauthID:       "",
		RequestTime:   int64(randomInRange(0, 10)),
		RawRequest:    "Qk9EWSBEQVRB",
		RawResponse:   "UkVTUE9OU0UgREFUQQ==",
		IPAddress:     "118.93.55.103",
		Tags:          []string{"orgid-" + orgID, "apiid-" + apiID},
		Alias:         "",
		TrackPath:     trackPath,
		ExpireAt:      time.Now().Add(time.Hour * 8760),
	}

	r.Geo.Country.ISOCode = country()

	return r
}
