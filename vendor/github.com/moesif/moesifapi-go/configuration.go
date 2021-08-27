/*
 * moesifapi-go
 */
package moesifapi

/** Version of this lib */
const Version string = "1.0.6"

type config struct {
	EventQueueSize int

	TimerWakeupSeconds int

	BatchSize int

	BaseURI string

	/** Your Application Id for authentication/authorization */
	/** Replace the value of ApplicationId with an appropriate value */
	MoesifApplicationId string
}

var Config = config{}
