/*
 * moesifapi-go
 */
package models

import "time"

/*
 * Structure for the custom type EventRequestModel
 */
type EventRequestModel struct {
	Time             *time.Time   `json:"time" form:"time"`                                               //Time when request was made
	Uri              string       `json:"uri" form:"uri"`                                                 //full uri of request such as https://www.example.com/my_path?param=1
	Verb             string       `json:"verb" form:"verb"`                                               //verb of the API request such as GET or POST
	Headers          interface{}  `json:"headers" form:"headers"`                                         //Key/Value map of request headers
	ApiVersion       *string      `json:"api_version,omitempty" form:"api_version,omitempty"`             //Optionally tag the call with your API or App version
	IpAddress        *string      `json:"ip_address,omitempty" form:"ip_address,omitempty"`               //IP Address of the client if known.
	Body             *interface{} `json:"body,omitempty" form:"body,omitempty"`                           //Request body
	TransferEncoding *string      `json:"transfer_encoding,omitempty" form:"transfer_encoding,omitempty"` //Transfer Encoding of Body, such as 'base64'
}

/*
 * Structure for the custom type EventModel
 */
type EventModel struct {
	Request      EventRequestModel  `json:"request" form:"request"`                                 //API request object
	Response     EventResponseModel `json:"response,omitempty" form:"response,omitempty"`           //API response Object
	SessionToken *string            `json:"session_token,omitempty" form:"session_token,omitempty"` //End user's auth/session token
	Tags         *string            `json:"tags,omitempty" form:"tags,omitempty"`                   //comma separated list of tags, see documentation
	UserId       *string            `json:"user_id,omitempty" form:"user_id,omitempty"`             //End user's user_id string from your app
	CompanyId    *string            `json:"company_id,omitempty" form:"company_id,omitempty"`       //company_id string
	Metadata	 interface{}		`json:"metadata,omitempty" form:"metadata,omitempty"`			//User Metadata
	Direction    *string            `json:"direction,omitempty" form:"direction,omitempty"`         // Direction of an API call
	Weight       *int               `json:"weight,omitempty" form:"weight,omitempty"`               // Weight of an API call
}

/*
 * Structure for the custom type EventResponseModel
 */
type EventResponseModel struct {
	Time             *time.Time  `json:"time" form:"time"`                                               //Time when response received
	Status           int         `json:"status" form:"status"`                                           //HTTP Status code such as 200
	Headers          interface{} `json:"headers" form:"headers"`                                         //Key/Value map of response headers
	Body             interface{} `json:"body" form:"body"`                                               //Response body
	IpAddress        *string     `json:"ip_address,omitempty" form:"ip_address,omitempty"`               //IP Address from the response, such as the server IP Address
	TransferEncoding *string     `json:"transfer_encoding,omitempty" form:"transfer_encoding,omitempty"` //Transfer Encoding of Body, such as 'base64'
}

/*
 * Structure for the custom type StatusModel
 */
type StatusModel struct {
	Status bool   `json:"status" form:"status"` //Status of Call
	Region string `json:"region" form:"region"` //Location
}

/*
 * Structure for the custom type CampaignModel
 */
 type CampaignModel struct {
	UtmSource			*string   	 `json:"utm_source,omitempty" form:"utm_source,omitempty"` 				//The Utm source
	UtmMedium    		*string      `json:"utm_medium,omitempty" form:"utm_medium,omitempty"` 				//The Utm Medium
	UtmCampaign	    	*string      `json:"utm_campaign,omitempty" form:"utm_campaign,omitempty"` 			//The Utm Campaign
	UtmTerm    			*string      `json:"utm_term,omitempty" form:"utm_term,omitempty"` 					//The Utm Term
	UtmContent			*string      `json:"utm_content,omitempty" form:"utm_content,omitempty"` 			//The Utm Content
	Referrer 			*string      `json:"referrer,omitempty" form:"referrer,omitempty"`					//The Referrer
	ReferringDomain	    *string	 	 `json:"referring_domain,omitempty" form:"referring_domain,omitempty"` 	//The Referring Domain
	Gclid    			*string      `json:"gclid,omitempty" form:"gclid,omitempty"` 						//The Gclid
 }

/*
 * Structure for the custom type UserModel
 */
 type UserModel struct {
	ModifiedTime	*time.Time     `json:"modified_time" form:"modified_time"` 								//Time when request was made
	SessionToken    *string        `json:"session_token,omitempty" form:"session_token,omitempty"` 			//End user's auth/session token
	IpAddress	    *string        `json:"ip_address,omitempty" form:"ip_address,omitempty"` 				//IP Address of the client if known.
	UserId    		string         `json:"user_id" form:"user_id"` 											//End user's user_id string from your app
	CompanyId		*string        `json:"company_id,omitempty" form:"company_id,omitempty"` 				//CompanyId associated with the user if known
	UserAgentString *string        `json:"user_agent_string,omitempty" form:"user_agent_string,omitempty"` 	//End user's user agent string
	Metadata	    interface{}	   `json:"metadata,omitempty" form:"metadata,omitempty"` 					//User Metadata
	Campaign     	*CampaignModel `json:"campaign,omitempty" form:"campaign,omitempty"`           			//The Campaign Object
 }

 /*
 * Structure for the custom type CompanyModel
 */
 type CompanyModel struct {
	ModifiedTime	*time.Time     `json:"modified_time" form:"modified_time"` 								//Time when request was made
	SessionToken    *string        `json:"session_token,omitempty" form:"session_token,omitempty"` 			//End user's auth/session token
	IpAddress	    *string        `json:"ip_address,omitempty" form:"ip_address,omitempty"` 				//IP Address of the client if known.
	CompanyId  		string         `json:"company_id" form:"company_id"` 									//Company Id string from your app
	CompanyDomain   *string        `json:"company_domain,omitempty" form:"company_domain,omitempty"` 		//Company Domain string
	Metadata	    interface{}	   `json:"metadata,omitempty" form:"metadata,omitempty"` 					//User Metadata
	Campaign     	*CampaignModel `json:"campaign,omitempty" form:"campaign,omitempty"`           			//The Campaign Object
 }