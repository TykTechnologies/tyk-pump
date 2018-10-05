package pumps

import (
	"encoding/json"

	"github.com/TykTechnologies/logrus"
	"github.com/TykTechnologies/tyk/rpc"
)

const hybridPrefix = "hybrid-pump"

type GroupLoginRequest struct {
	UserKey string
	GroupID string
}

var (
	dispatcherFuncs = map[string]interface{}{
		"Login": func(clientAddr, userKey string) bool {
			return false
		},
		"LoginWithGroup": func(clientAddr string, groupData *GroupLoginRequest) bool {
			return false
		},
		"PurgeAnalyticsData": func(data string) error {
			return nil
		},
		"Ping": func() bool {
			return false
		},
	}
)

// HybridPump allows to send analytics to MDCB over RPC
type HybridPump struct{}

func (p *HybridPump) GetName() string {
	return "Hybrid pump"
}

func (p *HybridPump) New() Pump {
	return &HybridPump{}
}

func (p *HybridPump) Init(config interface{}) error {
	meta := config.(map[string]interface{})

	// read configuration
	rpcConfig := rpc.Config{}
	if useSSL, ok := meta["use_ssl"]; ok {
		rpcConfig.UseSSL = useSSL.(bool)
	}
	if sslInsecure, ok := meta["ssl_insecure_skip_verify"]; ok {
		rpcConfig.SSLInsecureSkipVerify = sslInsecure.(bool)
	}
	if connStr, ok := meta["connection_string"]; ok {
		rpcConfig.ConnectionString = connStr.(string)
	} else {
		log.WithFields(logrus.Fields{
			"prefix": hybridPrefix,
		}).Fatal("Failed to decode configuration - no connection_string")
	}
	if rpcKey, ok := meta["rpc_key"]; ok {
		rpcConfig.RPCKey = rpcKey.(string)
	}
	if apiKey, ok := meta["api_key"]; ok {
		rpcConfig.APIKey = apiKey.(string)
	}
	if groupID, ok := meta["group_id"]; ok {
		rpcConfig.GroupID = groupID.(string)
	}
	if callTimeout, ok := meta["call_timeout"]; ok {
		rpcConfig.CallTimeout = int(callTimeout.(float64))
	}
	if pingTimeout, ok := meta["ping_timeout"]; ok {
		rpcConfig.PingTimeout = int(pingTimeout.(float64))
	}
	if rpcPoolSize, ok := meta["rpc_pool_size"]; ok {
		rpcConfig.RPCPoolSize = int(rpcPoolSize.(float64))
	}

	connected := rpc.Connect(
		rpcConfig,
		false,
		dispatcherFuncs,
		func(userKey string, groupID string) interface{} {
			return GroupLoginRequest{
				UserKey: userKey,
				GroupID: groupID,
			}
		},
		nil,
		nil,
	)
	if !connected {
		log.WithFields(logrus.Fields{
			"prefix": hybridPrefix,
		}).Fatal("Failed to connect to RPC server")
	}

	return nil
}

func (p *HybridPump) WriteData(data []interface{}) error {
	if len(data) == 0 {
		return nil
	}

	if _, err := rpc.FuncClientSingleton("Ping", nil); err != nil {
		log.WithFields(logrus.Fields{
			"prefix": hybridPrefix,
		}).WithError(err).Error("Failed to ping RPC server")
		return err
	}

	// turn array with analytics records into JSON payload
	jsonData, err := json.Marshal(data)
	if err != nil {
		log.WithFields(logrus.Fields{
			"prefix": hybridPrefix,
		}).WithError(err).Error("Failed to marshal analytics data")
		return err
	}

	// do RPC call to server
	if _, err := rpc.FuncClientSingleton("PurgeAnalyticsData", string(jsonData)); err != nil {
		log.WithFields(logrus.Fields{
			"prefix": hybridPrefix,
		}).WithError(err).Error("Failed to call PurgeAnalyticsData")
		return err
	}

	return nil
}
