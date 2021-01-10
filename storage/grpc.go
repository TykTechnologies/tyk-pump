package storage

import (
	"context"
	"fmt"
	"io"
	"net"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/TykTechnologies/tyk-pump/analyticspb"
	"github.com/enriquebris/goconcurrentqueue"
	"golang.org/x/sync/errgroup"
	"google.golang.org/grpc"
	"google.golang.org/grpc/keepalive"
	"github.com/TykTechnologies/logrus"

)
const DEFAULT_GRPC_PORT = 50051
const MIN_BUF_SIZE = 100000
var grpcLogPrefix = "grpc"

type server struct{
	workerCh chan *analyticspb.AnalyticsRecord
	queue *goconcurrentqueue.FIFO
}

type GrpcBuffer struct{
	grpcServer *grpc.Server
	server *server
	port int

}

func (s *GrpcBuffer) Init(config interface{}) error {
	s.port = DEFAULT_GRPC_PORT

	s.server = &server{}
	s.server.workerCh = make(chan *analyticspb.AnalyticsRecord, MIN_BUF_SIZE)
	s.server.queue = goconcurrentqueue.NewFIFO()
	go s.bufferWorker()
	go s.serveGrpc()


	return nil
}
func (s *GrpcBuffer) GetName() string {
	return "grpc"
}
func (s *GrpcBuffer) Connect() bool {

	return true
}
func (s *GrpcBuffer) GetAndDeleteSet(setName string, chunkSize int64, expire time.Duration) []interface{}{
	result := []interface{}{}


	for i:=0;i< s.server.queue.GetLen();i++{
		item, _  :=s.server.queue.Dequeue()
		result = append(result, item)
	}



	return result
}

func (s *GrpcBuffer) bufferWorker(){
	for {
		select {
			case val := <- s.server.workerCh:
				s.server.queue.Enqueue(val)
		}
	}

}


func (s *GrpcBuffer) serveGrpc(){
	ctx := context.Background()
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	interrupt := make(chan os.Signal, 1)
	signal.Notify(interrupt, os.Interrupt, syscall.SIGTERM)
	defer signal.Stop(interrupt)

	g, ctx := errgroup.WithContext(ctx)
	g.Go(func() error {
		addr := fmt.Sprintf(":%d", s.port)
		ln, err := net.Listen("tcp", addr)
		if err != nil {
			os.Exit(2)
		}


		s.grpcServer = grpc.NewServer(
			// MaxConnectionAge is just to avoid long connection, to facilitate load balancing
			// MaxConnectionAgeGrace will torn them, default to infinity
			grpc.KeepaliveParams(keepalive.ServerParameters{MaxConnectionAge: 2 * time.Minute}),
		)


		//Register service analytics
		// ex: myservice.RegisterNyServiceServer(grpcServer, server)
		//
		analyticspb.RegisterAnalyticsServiceServer(s.grpcServer,s.server)

		log.WithFields(logrus.Fields{
			"prefix": grpcLogPrefix,
		}).Info(fmt.Sprintf("gRPC server serving at %s", addr))

		return s.grpcServer.Serve(ln)
	})

	select {
	case <-interrupt:
		break
	case <-ctx.Done():
		break
	}

	log.WithFields(logrus.Fields{
		"prefix": grpcLogPrefix,
	}).Debug("Received shutdown signal.")

	if s.grpcServer != nil {
		s.grpcServer.GracefulStop()
	}
	err := g.Wait()
	if err != nil {
		log.WithFields(logrus.Fields{
			"prefix": grpcLogPrefix,
		}).Error( "server returning an error:", err)
		os.Exit(2)
	}
}

func (srv *server) SendData(stream analyticspb.AnalyticsService_SendDataServer) error{
	log.Printf("Pump receiving data via SendData! ")


	for  {
		record, err := stream.Recv()
		if err == io.EOF{
			return stream.SendAndClose(&analyticspb.AnalyticsRecordResp{
				Response: true,
			})
		}
		if err != nil {
			log.Fatalf("Error while reading client stream: %v",err)
			return err
		}
		srv.workerCh <- record
	}

	return nil
}

