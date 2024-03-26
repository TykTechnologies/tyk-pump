module github.com/TykTechnologies/tyk-pump

go 1.21

require (
	github.com/DataDog/datadog-go v4.7.0+incompatible
	github.com/TykTechnologies/gorpc v0.0.0-20210624160652-fe65bda0ccb9
	github.com/TykTechnologies/murmur3 v0.0.0-20230310161213-aad17efd5632
	github.com/TykTechnologies/storage v1.2.0
	github.com/aws/aws-sdk-go-v2 v1.22.1
	github.com/aws/aws-sdk-go-v2/config v1.9.0
	github.com/aws/aws-sdk-go-v2/credentials v1.5.0
	github.com/aws/aws-sdk-go-v2/service/sqs v1.26.0
	github.com/aws/aws-sdk-go-v2/service/timestreamwrite v1.9.0
	github.com/cenkalti/backoff/v4 v4.0.2
	github.com/elastic/go-elasticsearch/v8 v8.12.0
	github.com/fatih/structs v1.1.0
	github.com/gocraft/health v0.0.0-20170925182251-8675af27fef0
	github.com/gofrs/uuid v4.0.0+incompatible
	github.com/golang/protobuf v1.5.3
	github.com/google/go-cmp v0.6.0
	github.com/gorilla/mux v1.8.0
	github.com/influxdata/influxdb v1.8.10
	github.com/influxdata/influxdb-client-go/v2 v2.6.0
	github.com/kelseyhightower/envconfig v1.4.0
	github.com/logzio/logzio-go v0.0.0-20200316143903-ac8fc0e2910e
	github.com/mitchellh/mapstructure v1.3.1
	github.com/moesif/moesifapi-go v1.0.6
	github.com/oklog/ulid/v2 v2.1.0
	github.com/olivere/elastic/v7 v7.0.28
	github.com/oschwald/maxminddb-golang v1.11.0
	github.com/pkg/errors v0.9.1
	github.com/prometheus/client_golang v1.16.0
	github.com/quipo/statsd v0.0.0-20160923160612-75b7afedf0d2
	github.com/resurfaceio/logger-go/v3 v3.3.2
	github.com/robertkowalski/graylog-golang v0.0.0-20151121031040-e5295cfa2827
	github.com/segmentio/analytics-go v0.0.0-20160711225931-bdb0aeca8a99
	github.com/segmentio/kafka-go v0.3.6
	github.com/sirupsen/logrus v1.8.1
	github.com/stretchr/testify v1.8.4
	golang.org/x/net v0.17.0
	google.golang.org/protobuf v1.30.0
	gopkg.in/alecthomas/kingpin.v2 v2.2.6
	gopkg.in/olivere/elastic.v3 v3.0.56
	gopkg.in/olivere/elastic.v5 v5.0.85
	gopkg.in/olivere/elastic.v6 v6.2.31
	gopkg.in/vmihailenco/msgpack.v2 v2.9.1
	gorm.io/driver/mysql v1.0.3
	gorm.io/driver/postgres v1.2.0
	gorm.io/driver/sqlite v1.1.3
	gorm.io/gorm v1.21.16
)

require (
	github.com/Microsoft/go-winio v0.5.2 // indirect
	github.com/StackExchange/wmi v0.0.0-20190523213315-cbe66965904d // indirect
	github.com/alecthomas/template v0.0.0-20190718012654-fb15b899a751 // indirect
	github.com/alecthomas/units v0.0.0-20211218093645-b94a6e3cc137 // indirect
	github.com/andybalholm/brotli v1.0.5 // indirect
	github.com/asaskevich/govalidator v0.0.0-20210307081110-f21760c49a8d // indirect
	github.com/aws/aws-sdk-go-v2/feature/ec2/imds v1.7.0 // indirect
	github.com/aws/aws-sdk-go-v2/internal/configsources v1.2.1 // indirect
	github.com/aws/aws-sdk-go-v2/internal/endpoints/v2 v2.5.1 // indirect
	github.com/aws/aws-sdk-go-v2/internal/ini v1.2.5 // indirect
	github.com/aws/aws-sdk-go-v2/service/internal/endpoint-discovery v1.3.3 // indirect
	github.com/aws/aws-sdk-go-v2/service/internal/presigned-url v1.4.0 // indirect
	github.com/aws/aws-sdk-go-v2/service/sso v1.5.0 // indirect
	github.com/aws/aws-sdk-go-v2/service/sts v1.8.0 // indirect
	github.com/aws/smithy-go v1.16.0 // indirect
	github.com/beeker1121/goque v0.0.0-20170321141813-4044bc29b280 // indirect
	github.com/beorn7/perks v1.0.1 // indirect
	github.com/bmizerany/assert v0.0.0-20160611221934-b7ed37b82869 // indirect
	github.com/cespare/xxhash/v2 v2.2.0 // indirect
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/deepmap/oapi-codegen v1.8.2 // indirect
	github.com/dgryski/go-rendezvous v0.0.0-20200823014737-9f7001d12a5f // indirect
	github.com/elastic/elastic-transport-go/v8 v8.4.0 // indirect
	github.com/go-logr/logr v1.3.0 // indirect
	github.com/go-logr/stdr v1.2.2 // indirect
	github.com/go-ole/go-ole v1.2.4 // indirect
	github.com/go-sql-driver/mysql v1.5.0 // indirect
	github.com/golang/snappy v0.0.3 // indirect
	github.com/helloeave/json v1.15.3 // indirect
	github.com/influxdata/line-protocol v0.0.0-20200327222509-2487e7298839 // indirect
	github.com/jackc/chunkreader/v2 v2.0.1 // indirect
	github.com/jackc/pgconn v1.10.0 // indirect
	github.com/jackc/pgio v1.0.0 // indirect
	github.com/jackc/pgpassfile v1.0.0 // indirect
	github.com/jackc/pgproto3/v2 v2.1.1 // indirect
	github.com/jackc/pgservicefile v0.0.0-20200714003250-2b9c44734f2b // indirect
	github.com/jackc/pgtype v1.8.1 // indirect
	github.com/jackc/pgx/v4 v4.13.0 // indirect
	github.com/jehiah/go-strftime v0.0.0-20151206194810-2efbe75097a5 // indirect
	github.com/jinzhu/inflection v1.0.0 // indirect
	github.com/jinzhu/now v1.1.2 // indirect
	github.com/joho/godotenv v1.4.0 // indirect
	github.com/josharian/intern v1.0.0 // indirect
	github.com/klauspost/compress v1.13.6 // indirect
	github.com/kr/text v0.2.0 // indirect
	github.com/lib/pq v1.10.6 // indirect
	github.com/lintianzhi/graylogd v0.0.0-20180503131252-dc68342f04dc // indirect
	github.com/mailru/easyjson v0.7.7 // indirect
	github.com/mattn/go-sqlite3 v1.14.3 // indirect
	github.com/matttproud/golang_protobuf_extensions v1.0.4 // indirect
	github.com/montanaflynn/stats v0.0.0-20171201202039-1bf9dbcd8cbe // indirect
	github.com/olivere/elastic v6.2.31+incompatible // indirect
	github.com/onsi/gomega v1.20.0 // indirect
	github.com/pierrec/lz4 v2.6.0+incompatible // indirect
	github.com/pmezard/go-difflib v1.0.0 // indirect
	github.com/prometheus/client_model v0.3.0 // indirect
	github.com/prometheus/common v0.42.0 // indirect
	github.com/prometheus/procfs v0.10.1 // indirect
	github.com/redis/go-redis/v9 v9.3.1 // indirect
	github.com/rogpeppe/go-internal v1.11.0 // indirect
	github.com/segmentio/backo-go v0.0.0-20160424052352-204274ad699c // indirect
	github.com/shirou/gopsutil v3.20.11+incompatible // indirect
	github.com/syndtr/goleveldb v0.0.0-20190318030020-c3a204f8e965 // indirect
	github.com/xdg-go/pbkdf2 v1.0.0 // indirect
	github.com/xdg-go/scram v1.1.2 // indirect
	github.com/xdg-go/stringprep v1.0.4 // indirect
	github.com/xdg/scram v1.0.3 // indirect
	github.com/xdg/stringprep v1.0.3 // indirect
	github.com/xtgo/uuid v0.0.0-20140804021211-a0b114877d4c // indirect
	github.com/youmark/pkcs8 v0.0.0-20181117223130-1be2e3e5546d // indirect
	go.mongodb.org/mongo-driver v1.13.1 // indirect
	go.opentelemetry.io/otel v1.21.0 // indirect
	go.opentelemetry.io/otel/metric v1.21.0 // indirect
	go.opentelemetry.io/otel/trace v1.21.0 // indirect
	go.uber.org/atomic v1.9.0 // indirect
	golang.org/x/crypto v0.14.0 // indirect
	golang.org/x/sync v0.2.0 // indirect
	golang.org/x/sys v0.14.0 // indirect
	golang.org/x/text v0.13.0 // indirect
	google.golang.org/appengine v1.6.7 // indirect
	gopkg.in/mgo.v2 v2.0.0-20190816093944-a6b53ec6cb22 // indirect
	gopkg.in/yaml.v2 v2.4.0 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)

//replace gorm.io/gorm => ../gorm
replace gorm.io/gorm => github.com/TykTechnologies/gorm v1.20.7-0.20210910090358-06148e82dc85
