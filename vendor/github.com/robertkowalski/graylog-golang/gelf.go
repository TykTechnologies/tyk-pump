package gelf

import (
	"bytes"
	"compress/zlib"
	"crypto/rand"
	"encoding/binary"
	"encoding/json"
	"errors"
	"log"
	"math"
	"net"
	"strconv"
)

const (
	defaultGraylogPort     = 12201
	defaultGraylogHostname = "127.0.0.1"
	defaultConnection      = "wan"
	defaultMaxChunkSizeWan = 1420
	defaultMaxChunkSizeLan = 8154
)

type Config struct {
	GraylogPort     int
	GraylogHostname string
	Connection      string
	MaxChunkSizeWan int
	MaxChunkSizeLan int
}

type Gelf struct {
	Config
}

func New(config Config) *Gelf {

	if config.GraylogPort == 0 {
		config.GraylogPort = defaultGraylogPort
	}
	if config.GraylogHostname == "" {
		config.GraylogHostname = defaultGraylogHostname
	}
	if config.Connection == "" {
		config.Connection = defaultConnection
	}
	if config.MaxChunkSizeWan == 0 {
		config.MaxChunkSizeWan = defaultMaxChunkSizeWan
	}
	if config.MaxChunkSizeLan == 0 {
		config.MaxChunkSizeLan = defaultMaxChunkSizeLan
	}

	g := &Gelf{
		Config: config,
	}

	return g
}

func (g *Gelf) Log(message string) {
	msgJson := g.ParseJson(message)

	err := g.TestForForbiddenValues(msgJson)
	if err != nil {
		log.Printf("Uh oh! %s", err)
		return
	}

	compressed := g.Compress([]byte(message))

	chunksize := g.Config.MaxChunkSizeWan
	length := compressed.Len()

	if length > chunksize {

		chunkCountInt := int(math.Ceil(float64(length) / float64(chunksize)))

		id := make([]byte, 8)
		rand.Read(id)

		for i, index := 0, 0; i < length; i, index = i+chunksize, index+1 {
			packet := g.CreateChunkedMessage(index, chunkCountInt, id, &compressed)
			g.Send(packet.Bytes())
		}

	} else {
		g.Send(compressed.Bytes())
	}
}

func (g *Gelf) CreateChunkedMessage(index int, chunkCountInt int, id []byte, compressed *bytes.Buffer) bytes.Buffer {
	var packet bytes.Buffer

	chunksize := g.GetChunksize()

	packet.Write(g.IntToBytes(30))
	packet.Write(g.IntToBytes(15))
	packet.Write(id)

	packet.Write(g.IntToBytes(index))
	packet.Write(g.IntToBytes(chunkCountInt))

	packet.Write(compressed.Next(chunksize))

	return packet
}

func (g *Gelf) GetChunksize() int {

	if g.Config.Connection == "wan" {
		return g.Config.MaxChunkSizeWan
	}

	if g.Config.Connection == "lan" {
		return g.Config.MaxChunkSizeLan
	}

	return g.Config.MaxChunkSizeWan
}

func (g *Gelf) IntToBytes(i int) []byte {
	buf := new(bytes.Buffer)

	err := binary.Write(buf, binary.LittleEndian, int8(i))
	if err != nil {
		log.Printf("Uh oh! %s", err)
	}
	return buf.Bytes()
}

func (g *Gelf) Compress(b []byte) bytes.Buffer {
	var buf bytes.Buffer
	comp := zlib.NewWriter(&buf)

	comp.Write(b)
	comp.Close()

	return buf
}

func (g *Gelf) ParseJson(msg string) map[string]interface{} {
	var i map[string]interface{}
	c := []byte(msg)

	json.Unmarshal(c, &i)

	return i
}

func (g *Gelf) TestForForbiddenValues(gmap map[string]interface{}) error {
	if _, err := gmap["_id"]; err {
		return errors.New("Key _id is forbidden")
	}

	return nil
}

func (g *Gelf) Send(b []byte) {
	var addr = g.Config.GraylogHostname + ":" + strconv.Itoa(g.Config.GraylogPort)
	udpAddr, err := net.ResolveUDPAddr("udp", addr)
	if err != nil {
		log.Printf("Uh oh! %s", err)
		return
	}
	conn, err := net.DialUDP("udp", nil, udpAddr)
	if err != nil {
		log.Printf("Uh oh! %s", err)
		return
	}
	conn.Write(b)
}
