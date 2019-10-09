/*
 * moesifapi-go
 */
package moesifapi

import (
	"github.com/moesif/moesifapi-go/models"
)

/*
 * Interface for the Client
 */
type API interface {

	/**
	 * Queue Single API Event Call to be created
	 * @param    *models.EventModel        body     parameter: Required
	 * @return	Returns the  response from the API call
	 */
	QueueEvent(*models.EventModel) error

	/**
	 * Queue multiple API Events to be added
	 * @param    []*models.EventModel        body     parameter: Required
	 * @return	Returns the  response from the API call
	 */
	QueueEvents([]*models.EventModel) error

	/**
	 * Add Single API Event Call
	 * @param    *models.EventModel        body     parameter: Required
	 * @return	Returns the  response from the API call
	 */
	CreateEvent(*models.EventModel) error

	/**
	 * Add multiple API Events in a single batch (batch size must be less than 250kb)
	 * @param    []*models.EventModel        body     parameter: Required
	 * @return	Returns the  response from the API call
	 */
	CreateEventsBatch([]*models.EventModel) error

	Flush()

	Close()
}

/*
 * Factory for the API interaface returning Client
 */
func NewAPI(moesifApplicationId string) API {
	Config.MoesifApplicationId = moesifApplicationId
	return NewClient()
}
