package mongo

import "regexp"

const (
	_   = iota // ignore zero iota
	KiB = 1 << (10 * iota)
	MiB
	GiB
	TiB
)

// @PumpConf Mongo
type MongoConf struct {
	// TYKCONFIGEXPAND
	BaseMongoConf

	// Specifies the mongo collection name.
	CollectionName string `json:"collection_name" mapstructure:"collection_name"`
	// Maximum insert batch size for mongo selective pump. If the batch we are writing surpass this value, it will be send in multiple batchs.
	// Defaults to 10Mb.
	MaxInsertBatchSizeBytes int `json:"max_insert_batch_size_bytes" mapstructure:"max_insert_batch_size_bytes"`
	// Maximum document size. If the document exceed this value, it will be skipped.
	// Defaults to 10Mb.
	MaxDocumentSizeBytes int `json:"max_document_size_bytes" mapstructure:"max_document_size_bytes"`
	// Amount of bytes of the capped collection in 64bits architectures.
	// Defaults to 5GB.
	CollectionCapMaxSizeBytes int `json:"collection_cap_max_size_bytes" mapstructure:"collection_cap_max_size_bytes"`
	// Enable collection capping. It's used to set a maximum size of the collection.
	CollectionCapEnable bool `json:"collection_cap_enable" mapstructure:"collection_cap_enable"`
}

type BaseMongoConf struct {
	EnvPrefix string `mapstructure:"meta_env_prefix"`
	// The full URL to your MongoDB instance, this can be a clustered instance if necessary and
	// should include the database and username / password data.
	MongoURL string `json:"mongo_url" mapstructure:"mongo_url"`
	// Set to true to enable Mongo SSL connection.
	MongoUseSSL bool `json:"mongo_use_ssl" mapstructure:"mongo_use_ssl"`
	// Allows the use of self-signed certificates when connecting to an encrypted MongoDB database.
	MongoSSLInsecureSkipVerify bool `json:"mongo_ssl_insecure_skip_verify" mapstructure:"mongo_ssl_insecure_skip_verify"`
	// Ignore hostname check when it differs from the original (for example with SSH tunneling).
	// The rest of the TLS verification will still be performed.
	MongoSSLAllowInvalidHostnames bool `json:"mongo_ssl_allow_invalid_hostnames" mapstructure:"mongo_ssl_allow_invalid_hostnames"`
	// Path to the PEM file with trusted root certificates
	MongoSSLCAFile string `json:"mongo_ssl_ca_file" mapstructure:"mongo_ssl_ca_file"`
	// Path to the PEM file which contains both client certificate and private key. This is
	// required for Mutual TLS.
	MongoSSLPEMKeyfile string `json:"mongo_ssl_pem_keyfile" mapstructure:"mongo_ssl_pem_keyfile"`
	// Specifies the mongo DB Type. If it's 0, it means that you are using standard mongo db, but if it's 1 it means you are using AWS Document DB.
	// Defaults to Standard mongo (0).
	MongoDBType MongoType `json:"mongo_db_type" mapstructure:"mongo_db_type"`
	// Set to true to disable the default tyk index creation.
	OmitIndexCreation bool `json:"omit_index_creation" mapstructure:"omit_index_creation"`
}

func (c *BaseMongoConf) GetBlurredURL() string {
	// mongo uri match with regex ^(mongodb:(?:\/{2})?)((\w+?):(\w+?)@|:?@?)(\S+?):(\d+)(\/(\S+?))?(\?replicaSet=(\S+?))?$
	// but we need only a segment, so regex explanation: https://regex101.com/r/E34wQO/1
	regex := `^(mongodb:(?:\/{2})?)((\w+?):(\w+?)@|:?@?)`
	var re = regexp.MustCompile(regex)

	blurredUrl := re.ReplaceAllString(c.MongoURL, "***:***@")
	return blurredUrl
}
