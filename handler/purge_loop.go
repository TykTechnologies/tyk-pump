package handler

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/TykTechnologies/tyk-pump/analytics"
	"github.com/TykTechnologies/tyk-pump/instrumentation"
	"github.com/TykTechnologies/tyk-pump/pumps"
	"github.com/TykTechnologies/tyk-pump/storage"
	"github.com/gocraft/health"
	"gopkg.in/vmihailenco/msgpack.v2"
)


func (handler *PumpHandler) PurgeLoop() {
	handler.log.Infof("Starting purge loop @%d, chunk size %d", handler.SystemConfig.PurgeDelay, handler.SystemConfig.PurgeChunk)

	secInterval:= handler.SystemConfig.PurgeDelay
	chunkSize := handler.SystemConfig.PurgeChunk
	expire := time.Duration(handler.SystemConfig.StorageExpirationTime)*time.Second
	omitDetails := handler.SystemConfig.OmitDetailedRecording


	for range time.Tick(time.Duration(secInterval) * time.Second) {
		job := instrumentation.Instrument.NewJob("PumpRecordsPurge")

		AnalyticsValues := handler.AnalyticsStorage.GetAndDeleteSet(storage.ANALYTICS_KEYNAME, chunkSize, expire)
		if len(AnalyticsValues) > 0 {
			startTime := time.Now()

			// Convert to something clean
			keys := make([]interface{}, len(AnalyticsValues))

			for i, v := range AnalyticsValues {
				if v == nil {
					continue
				}

				decoded := analytics.AnalyticsRecord{}

				var err error

				switch v.(type){
				case analytics.AnalyticsRecord:
					decoded = v.(analytics.AnalyticsRecord)
				case *analytics.AnalyticsRecord:
					aux, ok := v.(*analytics.AnalyticsRecord)
					if !ok {
						err = errors.New("analytic record couldn't be decoded")
					}
					decoded = *aux
				default:
					err = msgpack.Unmarshal([]byte(v.(string)), &decoded)
					handler.log.Debug("Decoded Record: ", decoded)
				}


				if err != nil {
					handler.log.Error("Couldn't unmarshal analytics data:", err)
				} else {
					if omitDetails {
						decoded.RawRequest = ""
						decoded.RawResponse = ""
					}
					keys[i] = interface{}(decoded)
					job.Event("record")
				}
			}

			// Send to pumps
			handler.WriteToPumps(keys, job, startTime, int(secInterval))

			job.Timing("purge_time_all", time.Since(startTime).Nanoseconds())
		}

		if !handler.SystemConfig.DontPurgeUptimeData {
			UptimeValues := handler.UptimeStorage.GetAndDeleteSet(storage.UptimeAnalytics_KEYNAME, chunkSize, expire)
			handler.UptimePump.WriteUptimeData(UptimeValues)
		}
	}
}

func (handler *PumpHandler) WriteToPumps(keys []interface{}, job *health.Job, startTime time.Time, purgeDelay int) {
	// Send to pumps
	if handler.Pumps != nil {
		var wg sync.WaitGroup
		wg.Add(len(handler.Pumps))
		for _, pmp := range handler.Pumps {
			go handler.execPumpWriting(&wg, pmp, &keys, purgeDelay, startTime, job)
		}
		wg.Wait()
	} else {
		handler.log.Warning("No pumps defined!")
	}
}

func (handler *PumpHandler) execPumpWriting(wg *sync.WaitGroup, pmp pumps.Pump, keys *[]interface{}, purgeDelay int, startTime time.Time, job *health.Job) {
	timer := time.AfterFunc(time.Duration(purgeDelay)*time.Second, func() {
		if pmp.GetTimeout() == 0 {
			handler.log.Warning("Pump  ", pmp.GetName(), " is taking more time than the value configured of purge_delay. You should try to set a timeout for this pump.")
		} else if pmp.GetTimeout() > purgeDelay {
			handler.log.Warning("Pump  ", pmp.GetName(), " is taking more time than the value configured of purge_delay. You should try lowering the timeout configured for this pump.")
		}
	})
	defer timer.Stop()
	defer wg.Done()

	handler.log.Debug("Writing to: ", pmp.GetName())

	ch := make(chan error, 1)
	//Load pump timeout
	timeout := pmp.GetTimeout()
	var ctx context.Context
	var cancel context.CancelFunc
	//Initialize context depending if the pump has a configured timeout
	if timeout > 0 {
		ctx, cancel = context.WithTimeout(context.Background(), time.Duration(timeout)*time.Second)
	} else {
		ctx, cancel = context.WithCancel(context.Background())
	}

	defer cancel()

	go func(ch chan error, ctx context.Context, pmp pumps.Pump, keys *[]interface{}) {
		filteredKeys := filterData(pmp, *keys)

		ch <- pmp.WriteData(ctx, filteredKeys)
	}(ch, ctx, pmp, keys)

	select {
	case err := <-ch:
		if err != nil {
			handler.log.Warning("Error Writing to: ", pmp.GetName(), " - Error:", err)
		}
	case <-ctx.Done():
		switch ctx.Err() {
		case context.Canceled:
			handler.log.Warning("The writing to ", pmp.GetName(), " have got canceled.")
		case context.DeadlineExceeded:
			handler.log.Warning("Timeout Writing to: ", pmp.GetName())
		}
	}
	if job != nil {
		job.Timing("purge_time_"+pmp.GetName(), time.Since(startTime).Nanoseconds())
	}
}

//filterData filters each analytic records for each pump. It check if the pump contains filters or omit_detailed_recorded is true.
func filterData(pump pumps.Pump, keys []interface{}) []interface{} {
	filters := pump.GetFilters()
	if !filters.HasFilter() && !pump.GetOmitDetailedRecording() {
		return keys
	}
	filteredKeys := keys[:]
	newLenght := 0

	for _, key := range filteredKeys {
		decoded := key.(analytics.AnalyticsRecord)
		if pump.GetOmitDetailedRecording() {
			decoded.RawRequest = ""
			decoded.RawResponse = ""
		}
		if filters.ShouldFilter(decoded) {
			continue
		}
		filteredKeys[newLenght] = decoded
		newLenght++
	}
	filteredKeys = filteredKeys[:newLenght]
	return filteredKeys
}
