/*
 * moesifapi-go
 */
package moesifapi

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"time"

	"github.com/moesif/moesifapi-go/models"

	"golang.org/x/net/context"
	"golang.org/x/net/context/ctxhttp"

	"fmt"
	"net/http"
)

/*
 * Client structure as interface implementation
 */
type Client struct {
	cancel   func()
	ctx      context.Context
	ch       chan []*models.EventModel
	flush    chan chan struct{}
	interval time.Duration
}

func NewClient() *Client {
	ctx, cancel := context.WithCancel(context.Background())

	Client := &Client{
		cancel:   cancel,
		ctx:      ctx,
		ch:       make(chan []*models.EventModel, Config.QueueSize),
		flush:    make(chan chan struct{}),
		interval: time.Second * 15,
	}

	go Client.start()

	return Client
}

/**
 * Queue Single API Event Call to be created
 * @param    *models.EventModel        body     parameter: Required
 * @return	Returns the  response from the API call
 */
func (c *Client) QueueEvent(e *models.EventModel) error {
	events := make([]*models.EventModel, 1)
	events[0] = e
	select {
	case c.ch <- events:
		return nil
	default:
		return fmt.Errorf("Unable to send event, queue is full.  Use a larger queue size or create more workers.")
	}
}

/**
 * Queue multiple API Events to be added
 * @param    []*models.EventModel        body     parameter: Required
 * @return	Returns the  response from the API call
 */
func (c *Client) QueueEvents(e []*models.EventModel) error {
	select {
	case c.ch <- e:
		return nil
	default:
		return fmt.Errorf("Unable to send event, queue is full.  Use a larger queue size or create more workers.")
	}
}

/**
 * Add Single API Event Call
 * @param    *models.EventModel        body     parameter: Required
 * @return	Returns the  response from the API call
 */
func (c *Client) CreateEvent(event *models.EventModel) error {
	body, err := json.Marshal(&event)
	if err != nil {
		return err
	}

	url := BaseURI + "/v1/events"

	ctx, _ := context.WithTimeout(context.Background(), time.Second*10)

	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)

	if _, err = gz.Write(body); err != nil {
		return fmt.Errorf("Unable to gzip body.")
	}
	if err = gz.Close(); err != nil {
		return fmt.Errorf("Unable to close gzip writer.")
	}

	req, err := http.NewRequest("POST", url, &buf)
	if err != nil {
		return err
	}

	req.Header.Set("Content-Type", "application/json; charset=utf-8")
	req.Header.Set("X-Moesif-Application-Id", Config.MoesifApplicationId)
	req.Header.Set("User-Agent", "moesifapi-go/"+Version)
	req.Header.Set("Content-Encoding", "gzip")

	resp, err := ctxhttp.Do(ctx, http.DefaultClient, req)

	if resp != nil {
		defer resp.Body.Close()
	}

	return err
}

/**
 * Add multiple API Events in a single batch (batch size must be less than 250kb)
 * @param    []*models.EventModel        body     parameter: Required
 * @return	Returns the  response from the API call
 */
func (c *Client) CreateEventsBatch(events []*models.EventModel) error {
	body, err := json.Marshal(&events)
	if err != nil {
		return err
	}

	url := BaseURI + "/v1/events/batch"

	ctx, _ := context.WithTimeout(context.Background(), time.Second*10)

	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)

	if _, err = gz.Write(body); err != nil {
		return fmt.Errorf("Unable to gzip body.")
	}
	if err = gz.Close(); err != nil {
		return fmt.Errorf("Unable to close gzip writer.")
	}

	req, err := http.NewRequest("POST", url, &buf)
	if err != nil {
		return err
	}

	req.Header.Set("Content-Type", "application/json; charset=utf-8")
	req.Header.Set("X-Moesif-Application-Id", Config.MoesifApplicationId)
	req.Header.Set("User-Agent", "moesifapi-go/"+Version)
	req.Header.Set("Content-Encoding", "gzip")

	resp, err := ctxhttp.Do(ctx, http.DefaultClient, req)

	if resp != nil {
		defer resp.Body.Close()
	}

	return err
}

func (c *Client) Flush() {
	ch := make(chan struct{})
	defer close(ch)

	c.flush <- ch
	<-ch
}

func (c *Client) Close() {
	c.Flush()
	c.cancel()
}

func (c *Client) start() {
	timer := time.NewTimer(c.interval)

	bufferSize := 256
	buffer := make([]*models.EventModel, bufferSize)
	index := 0

	for {
		timer.Reset(c.interval)

		select {
		case <-c.ctx.Done():
			return

		case <-timer.C:
			if index > 0 {
				c.CreateEventsBatch(buffer[0:index])
				index = 0
			}

		case v := <-c.ch:
			buffer = append(buffer[:index], append(v, buffer[index:]...)...)
			index += len(v)
			if index >= bufferSize {
				c.CreateEventsBatch(buffer[0:index])
				index = 0
			}

		case v := <-c.flush:
			if index > 0 {
				c.CreateEventsBatch(buffer[0:index])
				index = 0
			}
			v <- struct{}{}
		}
	}
}
