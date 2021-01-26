package storage

import (
	"time"

	"github.com/TykTechnologies/tyk-pump/analytics"
	"github.com/enriquebris/goconcurrentqueue"
)

const MIN_BUF_SIZE = 100000

type Queue struct{
	queue *goconcurrentqueue.FIFO
	workerCh chan *analytics.AnalyticsRecord

}

func (q *Queue) Init(config interface{}) error {

	q.workerCh = make(chan *analytics.AnalyticsRecord, MIN_BUF_SIZE)
	q.queue = goconcurrentqueue.NewFIFO()
	go q.storeBufferWorker()

	return nil
}
func (q *Queue) GetName() string {
	return "grpc"
}
func (q *Queue) Connect() bool {

	return true
}
func (q *Queue) GetAndDeleteSet(setName string, chunkSize int64, expire time.Duration) []interface{}{
	result := []interface{}{}

	for i:=0;i< q.queue.GetLen();i++{
		item, _  :=q.queue.Dequeue()
		result = append(result, item)
	}


	return result
}

func (q *Queue) storeBufferWorker(){
	for {
		select {
		case val := <- q.workerCh:
			q.queue.Enqueue(val)
		}
	}

}

func(q *Queue) SendData(data ...*analytics.AnalyticsRecord) {
	for _,record := range data {
		q.workerCh <- record
	}
}