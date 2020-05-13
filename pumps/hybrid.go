package pumps

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/TykTechnologies/tyk-pump/analytics"

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
		"PurgeAnalyticsDataAggregated": func(data string) error {
			return nil
		},
	}
)

// HybridPump allows to send analytics to MDCB over RPC
type HybridPump struct {
	aggregated             bool
	trackAllPaths          bool
	storeAnalyticPerMinute bool
	ignoreTagPrefixList    []string
	filters                analytics.AnalyticsFilters
	timeout                int
}

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

	// check if we need to send aggregated analytics
	if aggregated, ok := meta["aggregated"]; ok {
		p.aggregated = aggregated.(bool)
	}

	if p.aggregated {
		if trackAllPaths, ok := meta["track_all_paths"]; ok {
			p.trackAllPaths = trackAllPaths.(bool)
		}

		if storeAnalyticPerMinute, ok := meta["store_analytics_per_minute"]; ok {
			p.storeAnalyticPerMinute = storeAnalyticPerMinute.(bool)
		}

		if list, ok := meta["ignore_tag_prefix_list"]; ok {
			ignoreTagPrefixList := list.([]interface{})
			p.ignoreTagPrefixList = make([]string, len(ignoreTagPrefixList))
			for k, v := range ignoreTagPrefixList {
				p.ignoreTagPrefixList[k] = fmt.Sprint(v)
			}
		}

	}

	return nil
}

func (p *HybridPump) WriteData(ctx context.Context, data []interface{}) error {
	if len(data) == 0 {
		return nil
	}

	if _, err := rpc.FuncClientSingleton("Ping", nil); err != nil {
		log.WithFields(logrus.Fields{
			"prefix": hybridPrefix,
		}).WithError(err).Error("Failed to ping RPC server")
		return err
	}

	// do RPC call to server
	if !p.aggregated { // send analytics records as is
		// turn array with analytics records into JSON payload
		jsonData, err := json.Marshal(data)
		if err != nil {
			log.WithFields(logrus.Fields{
				"prefix": hybridPrefix,
			}).WithError(err).Error("Failed to marshal analytics data")
			return err
		}

		if _, err := rpc.FuncClientSingleton("PurgeAnalyticsData", string(jsonData)); err != nil {
			log.WithFields(logrus.Fields{
				"prefix": hybridPrefix,
			}).WithError(err).Error("Failed to call PurgeAnalyticsData")
			return err
		}
	} else { // send aggregated data
		// calculate aggregates
		aggregates := analytics.AggregateData(data, p.trackAllPaths, p.ignoreTagPrefixList, p.storeAnalyticPerMinute)

		// turn map with analytics aggregates into JSON payload
		jsonData, err := json.Marshal(aggregates)
		if err != nil {
			log.WithFields(logrus.Fields{
				"prefix": hybridPrefix,
			}).WithError(err).Error("Failed to marshal analytics aggregates data")
			return err
		}

		if _, err := rpc.FuncClientSingleton("PurgeAnalyticsDataAggregated", string(jsonData)); err != nil {
			log.WithFields(logrus.Fields{
				"prefix": hybridPrefix,
			}).WithError(err).Error("Failed to call PurgeAnalyticsDataAggregated")
			return err
		}
	}

	return nil
}

func (p *HybridPump) SetFilters(filters analytics.AnalyticsFilters) {
	p.filters = filters
}
func (p *HybridPump) GetFilters() analytics.AnalyticsFilters {
	return p.filters
}
func (p *HybridPump) SetTimeout(timeout int) {
	p.timeout = timeout
}

func (p *HybridPump) GetTimeout() int {
	return p.timeout
}
