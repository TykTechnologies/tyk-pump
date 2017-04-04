/*
 * moesifapi-go
 */
package moesifapi

/** Version of this lib */
const Version string = "2.1.2"

/** The base Uri for API calls */
const BaseURI string = "https://api.moesif.net"

type config struct {
	QueueSize int

	/** Your Application Id for authentication/authorization */
	/** Replace the value of ApplicationId with an appropriate value */
	MoesifApplicationId string
}

var Config = config{QueueSize: 256}
