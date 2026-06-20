// Test Helper for Mongo

package pumps

import (
	"context"
	"os"
	"regexp"
	"strings"
	"testing"

	"github.com/TykTechnologies/storage/persistent"
	"github.com/TykTechnologies/storage/persistent/model"
)

var nonAlnumRE = regexp.MustCompile(`[^A-Za-z0-9_]`)

// uniqueCollection produces a deterministic mongo-safe collection name keyed
// to the running test, so concurrent or repeated runs in the shared
// testcontainer don't collide.
func uniqueCollection(t *testing.T) string {
	t.Helper()
	name := nonAlnumRE.ReplaceAllString(t.Name(), "_")
	// Mongo collections must be < 120 bytes; keep well below.
	if len(name) > 90 {
		name = name[:90]
	}
	return "tyk_test_" + strings.ToLower(name)
}

// dbAddr is retained for the few unit tests that don't actually open a
// connection (e.g. GetBlurredURL tests).  All real tests use mongoConnectionURI(t).
const (
	dbAddr  = "mongodb://localhost:27017/test"
	colName = "test_collection"
)

// testMongoURI returns the live testcontainer URI when a *testing.T is provided,
// falling back to the static dbAddr only when no test handle is available.
// The testcontainer module returns a URI without a database segment; the
// persistent storage layer needs one, so we append /tyk_analytics when missing.
func testMongoURI(t *testing.T) string {
	if t == nil {
		return dbAddr
	}
	uri := mongoConnectionURI(t)
	return ensureMongoDatabase(uri, "tyk_analytics")
}

// ensureMongoDatabase appends `/db` to a mongo URI when the URI has no
// database path component. This is required because testcontainers-go's
// MongoDB module returns a URI like "mongodb://host:port" with no DB segment,
// but the TykTechnologies persistent storage layer requires one.
func ensureMongoDatabase(uri, db string) string {
	// Split on the scheme://host portion: find the host part end (third "/")
	if uri == "" {
		return uri
	}
	// uri form: scheme://host[:port][/path][?query]
	scheme := ""
	rest := uri
	if idx := strings.Index(uri, "://"); idx >= 0 {
		scheme = uri[:idx+3]
		rest = uri[idx+3:]
	}
	// Separate query from rest
	query := ""
	if idx := strings.Index(rest, "?"); idx >= 0 {
		query = rest[idx:]
		rest = rest[:idx]
	}
	// Look for first slash in rest -> indicates database path
	if idx := strings.Index(rest, "/"); idx >= 0 {
		// If there's something after the slash, leave URI alone
		if idx < len(rest)-1 {
			return uri
		}
		// trailing slash with no db -> append db
		return scheme + rest + db + query
	}
	return scheme + rest + "/" + db + query
}

type Conn struct {
	Store persistent.PersistentStorage
	URI   string
}

func (c *Conn) TableName() string {
	return colName
}

func (*Conn) GetObjectID() model.ObjectID {
	return ""
}

// SetObjectID is a dummy function to satisfy the interface
func (*Conn) SetObjectID(model.ObjectID) {
	// empty
}

func (c *Conn) ConnectDb(t *testing.T) {
	t.Helper()
	if c.Store == nil {
		uri := c.URI
		if uri == "" {
			uri = testMongoURI(t)
			c.URI = uri
		}
		var err error
		c.Store, err = persistent.NewPersistentStorage(&persistent.ClientOpts{
			Type:             "mongo-go",
			ConnectionString: uri,
		})
		if err != nil {
			t.Fatalf("Unable to connect to mongo: %v", err)
		}
	}
}

func (c *Conn) CleanDb() {
	if c.Store == nil {
		return
	}
	if err := c.Store.DropDatabase(context.Background()); err != nil {
		panic(err)
	}
}

func (c *Conn) CleanCollection() {
	err := c.Store.Drop(context.Background(), c)
	if err != nil {
		panic(err)
	}
}

func (c *Conn) CleanIndexes() {
	err := c.Store.CleanIndexes(context.Background(), c)
	if err != nil {
		panic(err)
	}
}

type Doc struct {
	ID  model.ObjectID `bson:"_id"`
	Foo string         `bson:"foo"`
}

func (d Doc) GetObjectID() model.ObjectID {
	return d.ID
}

// SetObjectID is a dummy function to satisfy the interface
func (d *Doc) SetObjectID(id model.ObjectID) {
	d.ID = id
}

func (d Doc) TableName() string {
	return colName
}

func (c *Conn) InsertDoc() {
	doc := Doc{
		Foo: "bar",
	}
	doc.SetObjectID(model.NewObjectID())
	err := c.Store.Insert(context.Background(), &doc)
	if err != nil {
		panic(err)
	}
}

func (c *Conn) GetCollectionStats() (colStats model.DBM) {
	var err error
	colStats, err = c.Store.DBTableStats(context.Background(), c)
	if err != nil {
		panic(err)
	}
	return colStats
}

func (c *Conn) GetIndexes() ([]model.Index, error) {
	return c.Store.GetIndexes(context.Background(), c)
}

// defaultConf returns a MongoConf wired to the testcontainer mongo URI.
func defaultConf(t *testing.T) MongoConf {
	conf := MongoConf{
		CollectionName:          colName,
		MaxInsertBatchSizeBytes: 10 * MiB,
		MaxDocumentSizeBytes:    10 * MiB,
	}

	conf.MongoURL = testMongoURI(t)
	conf.MongoSSLInsecureSkipVerify = true

	if os.Getenv("MONGO_DRIVER") == "mgo" {
		conf.MongoDriverType = persistent.Mgo
	} else {
		conf.MongoDriverType = persistent.OfficialMongo
	}

	return conf
}

func defaultSelectiveConf(t *testing.T) MongoSelectiveConf {
	conf := MongoSelectiveConf{
		MaxInsertBatchSizeBytes: 10 * MiB,
		MaxDocumentSizeBytes:    10 * MiB,
	}

	conf.MongoURL = testMongoURI(t)
	conf.MongoSSLInsecureSkipVerify = true

	if os.Getenv("MONGO_DRIVER") == "mgo" {
		conf.MongoDriverType = persistent.Mgo
	} else {
		conf.MongoDriverType = persistent.OfficialMongo
	}

	return conf
}
