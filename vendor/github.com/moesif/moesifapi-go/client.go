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
	chUser	 chan []*models.UserModel
	chCompany chan []*models.CompanyModel
	flush    chan chan struct{}
	interval time.Duration
	responseETag string
}

func (c *Client) GetETag() string {
	return c.responseETag
}

/*
 * Get X-Moesif-Config-Etag from response headers
 */
func (c *Client) fetchETagFromHeader(headers http.Header) {	
	if eTag, ok := headers["X-Moesif-Config-Etag"]; ok {
		c.responseETag = eTag[0]
	}
}

func NewClient() *Client {
	ctx, cancel := context.WithCancel(context.Background())

	Client := &Client{
		cancel:   cancel,
		ctx:      ctx,
		ch:       make(chan []*models.EventModel, Config.EventQueueSize),
		chUser:   make(chan []*models.UserModel, Config.EventQueueSize),
		chCompany: make(chan []*models.CompanyModel, Config.EventQueueSize),
		flush:    make(chan chan struct{}),
		interval: time.Second * time.Duration(Config.TimerWakeupSeconds),
		responseETag: "",
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
 * Queue Single User to be updated
 * @param    *models.UserModel        body     parameter: Required
 * @return	Returns the  response from the API call
 */
func (c *Client) QueueUser(u *models.UserModel) error {
	users := make([]*models.UserModel, 1)
	users[0] = u
	select {
	case c.chUser <- users:
		return nil
	default:
		return fmt.Errorf("Unable to send event, queue is full.  Use a larger queue size or create more workers.")
	}
}

/**
 * Queue multiple Users to be updated
 * @param    []*models.UserModel        body     parameter: Required
 * @return	Returns the  response from the API call
 */
func (c *Client) QueueUsers(u []*models.UserModel) error {
	select {
	case c.chUser <- u:
		return nil
	default:
		return fmt.Errorf("Unable to send event, queue is full.  Use a larger queue size or create more workers.")
	}
}

/**
 * Queue Single Company to be added
 * @param    *models.CompanyModel        body     parameter: Required
 * @return	Returns the  response from the API call
 */
 func (c *Client) QueueCompany(u *models.CompanyModel) error {
	companies := make([]*models.CompanyModel, 1)
	companies[0] = u
	select {
	case c.chCompany <- companies:
		return nil
	default:
		return fmt.Errorf("Unable to send event, queue is full.  Use a larger queue size or create more workers.")
	}
}

/**
 * Queue multiple companies to be added
 * @param    []*models.UserModel        body     parameter: Required
 * @return	Returns the  response from the API call
 */
 func (c *Client) QueueCompanies(u []*models.CompanyModel) error {
	select {
	case c.chCompany <- u:
		return nil
	default:
		return fmt.Errorf("Unable to send event, queue is full.  Use a larger queue size or create more workers.")
	}
}

/**
* Log data to Moesif
* @param    []bytes        body        parameter: Required
* @param    string         rawPath     parameter: Required
*/

func (c *Client) SendDataToMoesif(body []byte, rawPath string) () {

	url := Config.BaseURI + rawPath

	ctx, _ := context.WithTimeout(context.Background(), time.Second*10)

	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)

	if _, err := gz.Write(body); err != nil {
		_ = fmt.Errorf("Unable to gzip body.")
		return
	}
	if err := gz.Close(); err != nil {
		_ = fmt.Errorf("Unable to close gzip writer.")
		return
	}

	req, err := http.NewRequest("POST", url, &buf)
	if err != nil {
		return
	}

	req.Header.Set("Content-Type", "application/json; charset=utf-8")
	req.Header.Set("X-Moesif-Application-Id", Config.MoesifApplicationId)
	req.Header.Set("User-Agent", "moesifapi-go/"+Version)
	req.Header.Set("Content-Encoding", "gzip")
	
	resp, _ := ctxhttp.Do(ctx, http.DefaultClient, req)
	if resp != nil {
		defer resp.Body.Close()
		// Fetch X-Moesif-Config-Etag from Response headers
		c.fetchETagFromHeader(resp.Header)
	}
}

/**
 * Add Single API Event Call
 * @param    *models.EventModel        body     parameter: Required
 * @return	Returns the  response from the API call
 */
func (c *Client) CreateEvent(event *models.EventModel) (http.Header, error) {
	body, err := json.Marshal(&event)
	if err != nil {
		return nil, err
	}

	// Send data to Moesif async
	go c.SendDataToMoesif(body, "/v1/events")

	return nil, err
}


/**
 * Add multiple API Events in a single batch (batch size must be less than 250kb)
 * @param    []*models.EventModel        body     parameter: Required
 * @return	Returns the  response from the API call
 */
func (c *Client) CreateEventsBatch(events []*models.EventModel) (http.Header, error) {

	body, err := json.Marshal(&events)
	if err != nil {
		return nil, err
	}

	// Send data to Moesif async
	go c.SendDataToMoesif(body, "/v1/events/batch")

	return nil, err
}

/**
 * Update a Single User
 * @param    *models.UserModel        body     parameter: Required
 * @return	Returns the  response from the API call
 */
 func (c *Client) UpdateUser(user *models.UserModel) error {
	body, err := json.Marshal(&user)
	if err != nil {
		return err
	}

	// Send data to Moesif async
	go c.SendDataToMoesif(body, "/v1/users")

	return err
}


/**
 * Update multiple Users in a single batch (batch size must be less than 250kb)
 * @param    []*models.UserModel        body     parameter: Required
 * @return	Returns the  response from the API call
 */
 func (c *Client) UpdateUsersBatch(users []*models.UserModel) error {
	body, err := json.Marshal(&users)
	if err != nil {
		return err
	}

	// Send data to Moesif async
	go c.SendDataToMoesif(body, "/v1/users/batch")

	return err
}

/**
 * Get Application configuration
 * @param  nil  parameter: Required
 * @return	Returns the  response from the API call
 */
 func (c *Client) GetAppConfig() (*http.Response, error) {

	url := Config.BaseURI + "/v1/config"

	ctx, _ := context.WithTimeout(context.Background(), time.Second*10)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/json; charset=utf-8")
	req.Header.Set("X-Moesif-Application-Id", Config.MoesifApplicationId)

	resp, err := ctxhttp.Do(ctx, http.DefaultClient, req)

	if err != nil {
		return nil, err
	}

	return resp, nil
}

/**
 * Update a Single Company
 * @param    *models.CompanyModel        body     parameter: Required
 * @return	Returns the  response from the API call
 */
 func (c *Client) UpdateCompany(company *models.CompanyModel) error {
	body, err := json.Marshal(&company)
	if err != nil {
		return err
	}

	// Send data to Moesif async
	go c.SendDataToMoesif(body, "/v1/companies")

	return err
}


/**
 * Update multiple companies in a single batch (batch size must be less than 250kb)
 * @param    []*models.CompanyModel        body     parameter: Required
 * @return	Returns the  response from the API call
 */
 func (c *Client) UpdateCompaniesBatch(companies []*models.CompanyModel) error {
	body, err := json.Marshal(&companies)
	if err != nil {
		return err
	}

	// Send data to Moesif async
	go c.SendDataToMoesif(body, "/v1/companies/batch")

	return err
}

func (c *Client) Flush() {
	ch := make(chan struct{})
	defer close(ch)

	chUser := make(chan struct{})
	defer close(chUser)

	chCompany := make(chan struct{})
	defer close(chCompany)

	c.flush <- ch
	<-ch

	c.flush <- chUser
	<-chUser

	c.flush <- chCompany
	<-chCompany
}

func (c *Client) Close() {
	c.Flush()
	c.cancel()
}

func (c *Client) start() {
	timer := time.NewTimer(c.interval)

	bufferSize := Config.BatchSize
	buffer := make([]*models.EventModel, bufferSize)
	bufferUser := make([]*models.UserModel, bufferSize)
	bufferCompany := make([]*models.CompanyModel, bufferSize)
	index := 0
	indexUser := 0
	indexCompany := 0

	for {
		timer.Reset(c.interval)

		select {
		case <-c.ctx.Done():
			return

		case <-timer.C:
			if index > 0 {
				c.CreateEventsBatch(buffer[0:index])
				for i := 0; i < index; i++ {
					buffer[i] = nil
				}
				index = 0
			}
			if indexUser > 0 {
				c.UpdateUsersBatch(bufferUser[0:indexUser])
				for i := 0; i < indexUser; i++ {
					bufferUser[i] = nil
				}
				indexUser = 0
			}

			if indexCompany > 0 {
				c.UpdateCompaniesBatch(bufferCompany[0:indexCompany])
				for i := 0; i < indexCompany; i++ {
					bufferCompany[i] = nil
				}
				indexCompany = 0
			}

		case v := <-c.ch:
			for _, event := range v {
				buffer[index] = event
				index += 1

				if index >= bufferSize {
					c.CreateEventsBatch(buffer[0:index])
					for i := 0; i < index; i++ {
						buffer[i] = nil
					}
					index = 0
				}
			}

		case v := <-c.chUser:
			for _, user := range v {
				bufferUser[indexUser] = user
				indexUser += 1

				if indexUser >= bufferSize {
					c.UpdateUsersBatch(bufferUser[0:indexUser])
					for i := 0; i < indexUser; i++ {
						bufferUser[i] = nil
					}
					indexUser = 0
				}
			}	
		
		case v := <-c.chCompany:
			for _, company := range v {
				bufferCompany[indexCompany] = company
				indexCompany += 1

				if indexCompany >= bufferSize {
					c.UpdateCompaniesBatch(bufferCompany[0:indexCompany])
					for i := 0; i < indexCompany; i++ {
						bufferCompany[i] = nil
					}
					indexCompany = 0
				}
			}

		case v := <-c.flush:
			if index > 0 {
				c.CreateEventsBatch(buffer[0:index])
				for i := 0; i < index; i++ {
					buffer[i] = nil
				}
				index = 0
			}
			if indexUser > 0 {
				c.UpdateUsersBatch(bufferUser[0:indexUser])
				for i := 0; i < indexUser; i++ {
					bufferUser[i] = nil
				}
				indexUser = 0
			}
			if indexCompany > 0 {
				c.UpdateCompaniesBatch(bufferCompany[0:indexCompany])
				for i := 0; i < indexCompany; i++ {
					bufferCompany[i] = nil
				}
				indexCompany = 0
			}
			v <- struct{}{}
		}
	}
}
