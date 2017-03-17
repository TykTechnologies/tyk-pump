package demo

import (
	"time"
	"math/rand"
	"github.com/satori/go.uuid"
	"github.com/TykTechnologies/tyk-pump/analytics"
	"strings"
	"github.com/gocraft/health"
)

var apiKeys []string

func DemoInit(orgId string) {
	apiKeys = generateAPIKeys(orgId)
}

func randomInRange(min, max int) int {
	rand.Seed(time.Now().UnixNano())
	return rand.Intn(max - min) + min
}

func randomMethod() string {
	var methods []string = []string{"GET", "PUT", "POST", "DELETE", "OPTIONS", "HEAD"}

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
	for  i:=1; i <= depth; i++  {
		path += "/" + wordset[rand.Intn(len(wordset))]
	}

	return path
}

func randomAPI() (string, string) {
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

func generateAPIKeys(orgId string) []string {
	set := make([]string, 50)
	for i := 0; i < len(set); i++ {
		set[i] = generateAPIKey(orgId)
	}

	return set
}

func generateAPIKey(orgId string) string {
	u1 := uuid.NewV4()
	id := strings.Replace(u1.String(), "-", "", -1)
	return orgId + id
}

func getRandomKey() string {
	return apiKeys[rand.Intn(len(apiKeys))]
}

func GenerateDemoData(start time.Time, days int, orgId string, writer func([]interface{}, *health.Job, time.Time)) {
	count := 0
	finish := start.AddDate(0, 0, days)
	for d := start; d.Before(finish); d = d.AddDate(0, 0, 1) {
		set := []interface{}{}

		// Generate daily entries
		volume := randomInRange(100, 500)
		for i:= 0; i < volume; i++ {
			p := randomPath()
			api, apiID := randomAPI()
			r := analytics.AnalyticsRecord{
				Method: randomMethod(),
				Path: p,
				RawPath: p,
				ContentLength: int64(randomInRange(0, 999)),
				UserAgent: getUA(),
				Day: d.Day(),
				Month: d.Month(),
				Year: d.Year(),
				Hour: d.Hour(),
				ResponseCode: responseCode(),
				APIKey: getRandomKey(),
				TimeStamp: d,
				APIVersion: "Default",
				APIName: api,
				APIID: apiID,
				OrgID: orgId,
				OauthID: "",
				RequestTime: int64(randomInRange(0, 10)),
				RawRequest: "Qk9EWSBEQVRB",
				RawResponse: "UkVTUE9OU0UgREFUQQ==",
				IPAddress: "118.93.55.103",
				Tags: []string{"orgid-"+orgId, "apiid-"+apiID},
				Alias: "",
				TrackPath: true,
				ExpireAt: time.Now().Add(time.Hour * 8760),
			}



			set = append(set, r)
		}
		count += 1
		writer(set, nil, time.Now())

	}
}

