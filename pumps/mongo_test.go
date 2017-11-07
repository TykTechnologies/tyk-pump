package pumps

import (
	"strconv"
	"testing"
)

func newPump() Pump {
	return (&MongoPump{}).New()
}

func TestMongoPump_capCollection_Enabled(t *testing.T) {

	c := Conn{}
	c.ConnectDb()
	defer c.CleanDb()

	pump := newPump()
	conf := defaultConf()

	mPump := pump.(*MongoPump)
	mPump.dbConf = &conf
	mPump.dbConf.CollectionCapEnable = false

	mPump.connect()

	if ok := mPump.capCollection(); ok {
		t.Error("successfully capped collection when disabled in conf")
	}
}

func TestMongoPump_capCollection_Exists(t *testing.T) {

	c := Conn{}
	c.ConnectDb()
	defer c.CleanDb()

	c.InsertDoc()

	pump := newPump()
	conf := defaultConf()

	mPump := pump.(*MongoPump)
	mPump.dbConf = &conf

	mPump.dbConf.CollectionCapEnable = true

	mPump.connect()

	if ok := mPump.capCollection(); ok {
		t.Error("successfully capped collection when already exists")
	}
}

func TestMongoPump_capCollection_Not64arch(t *testing.T) {

	c := Conn{}
	c.ConnectDb()
	defer c.CleanDb()

	if strconv.IntSize >= 64 {
		t.Skip("skipping as >= 64bit arch")
	}

	pump := newPump()
	conf := defaultConf()

	mPump := pump.(*MongoPump)
	mPump.dbConf = &conf

	mPump.dbConf.CollectionCapEnable = true

	mPump.connect()

	if ok := mPump.capCollection(); ok {
		t.Error("should not be able to cap collection when running < 64bit architecture")
	}
}

func TestMongoPump_capCollection_SensibleDefaultSize(t *testing.T) {

	if strconv.IntSize < 64 {
		t.Skip("skipping as < 64bit arch")
	}

	c := Conn{}
	c.ConnectDb()
	defer c.CleanDb()

	pump := newPump()
	conf := defaultConf()

	mPump := pump.(*MongoPump)
	mPump.dbConf = &conf

	mPump.dbConf.CollectionCapEnable = true
	mPump.dbConf.CollectionCapMaxSizeBytes = 0

	mPump.connect()

	if ok := mPump.capCollection(); !ok {
		t.Fatal("should have capped collection")
	}

	colStats := c.GetCollectionStats()

	defSize := 5
	if colStats["maxSize"].(int64) != int64(defSize*GiB) {
		t.Errorf("wrong sized capped collection created. Expected (%d), got (%d)", mPump.dbConf.CollectionCapMaxSizeBytes, colStats["maxSize"])
	}
}

func TestMongoPump_capCollection_OverrideSize(t *testing.T) {

	if strconv.IntSize < 64 {
		t.Skip("skipping as < 64bit arch")
	}

	c := Conn{}
	c.ConnectDb()
	defer c.CleanDb()

	pump := newPump()
	conf := defaultConf()

	mPump := pump.(*MongoPump)
	mPump.dbConf = &conf

	mPump.dbConf.CollectionCapEnable = true
	mPump.dbConf.CollectionCapMaxSizeBytes = GiB

	mPump.connect()

	if ok := mPump.capCollection(); !ok {
		t.Error("should have capped collection")
		t.FailNow()
	}

	colStats := c.GetCollectionStats()

	if colStats["maxSize"].(int64) != int64(mPump.dbConf.CollectionCapMaxSizeBytes) {
		t.Errorf("wrong sized capped collection created. Expected (%d), got (%d)", mPump.dbConf.CollectionCapMaxSizeBytes, colStats["maxSize"])
	}
}
