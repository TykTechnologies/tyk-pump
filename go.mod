module github.com/TykTechnologies/tyk-pump

go 1.15

require (
	github.com/DataDog/datadog-go v4.7.0+incompatible
	github.com/StackExchange/wmi v0.0.0-20190523213315-cbe66965904d // indirect
	github.com/TykTechnologies/graphql-go-tools v1.6.2-0.20220811124354-8d1f142966f8
	github.com/TykTechnologies/murmur3 v0.0.0-20180602122059-1915e687e465
	github.com/TykTechnologies/tyk v0.0.0-20200207055804-cf1d1ad81206
	github.com/aws/aws-sdk-go-v2 v1.16.14
	github.com/aws/aws-sdk-go-v2/config v1.9.0
	github.com/aws/aws-sdk-go-v2/service/timestreamwrite v1.9.0
	github.com/beeker1121/goque v0.0.0-20170321141813-4044bc29b280 // indirect
	github.com/bmizerany/assert v0.0.0-20160611221934-b7ed37b82869 // indirect
	github.com/buger/jsonparser v1.1.1
	github.com/fatih/structs v1.1.0
	github.com/go-ole/go-ole v1.2.4 // indirect
	github.com/go-redis/redis/v8 v8.3.1
	github.com/gocraft/health v0.0.0-20170925182251-8675af27fef0
	github.com/golang/protobuf v1.5.2
	github.com/google/go-cmp v0.5.9
	github.com/gorilla/mux v1.8.0
	github.com/influxdata/influxdb v1.8.3
	github.com/influxdata/influxdb-client-go/v2 v2.6.0
	github.com/jehiah/go-strftime v0.0.0-20151206194810-2efbe75097a5 // indirect
	github.com/kelseyhightower/envconfig v1.4.0
	github.com/lintianzhi/graylogd v0.0.0-20180503131252-dc68342f04dc // indirect
	github.com/logzio/logzio-go v0.0.0-20200316143903-ac8fc0e2910e
	github.com/lonelycode/mgohacks v0.0.0-20150820024025-f9c291f7e57e
	github.com/mitchellh/mapstructure v1.2.2
	github.com/moesif/moesifapi-go v1.0.6
	github.com/olivere/elastic v6.2.31+incompatible // indirect
	github.com/olivere/elastic/v7 v7.0.28
	github.com/oschwald/maxminddb-golang v1.5.0
	github.com/pkg/errors v0.9.1
	github.com/prometheus/client_golang v1.11.0
	github.com/quipo/statsd v0.0.0-20160923160612-75b7afedf0d2
	github.com/robertkowalski/graylog-golang v0.0.0-20151121031040-e5295cfa2827
	github.com/satori/go.uuid v1.2.0
	github.com/segmentio/analytics-go v0.0.0-20160711225931-bdb0aeca8a99
	github.com/segmentio/backo-go v0.0.0-20160424052352-204274ad699c // indirect
	github.com/segmentio/kafka-go v0.3.6
	github.com/shirou/gopsutil v3.20.11+incompatible // indirect
	github.com/sirupsen/logrus v1.8.1
	github.com/stretchr/testify v1.8.0
	github.com/syndtr/goleveldb v0.0.0-20190318030020-c3a204f8e965 // indirect
	github.com/xtgo/uuid v0.0.0-20140804021211-a0b114877d4c // indirect
	golang.org/x/lint v0.0.0-20200302205851-738671d3881b // indirect
	golang.org/x/net v0.0.0-20210614182718-04defd469f4e
	google.golang.org/protobuf v1.27.1
	gopkg.in/alecthomas/kingpin.v2 v2.2.6
	gopkg.in/mgo.v2 v2.0.0-20190816093944-a6b53ec6cb22
	gopkg.in/olivere/elastic.v3 v3.0.56
	gopkg.in/olivere/elastic.v5 v5.0.85
	gopkg.in/olivere/elastic.v6 v6.2.31
	gopkg.in/vmihailenco/msgpack.v2 v2.9.1
	gorm.io/driver/mysql v1.0.3
	gorm.io/driver/postgres v1.0.5
	gorm.io/driver/sqlite v1.1.3
	gorm.io/gorm v1.21.10
)

//replace gorm.io/gorm => ../gorm
replace gorm.io/gorm => github.com/TykTechnologies/gorm v1.20.7-0.20210910090358-06148e82dc85
