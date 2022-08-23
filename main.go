package main

import (
	"fmt"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"google.golang.org/grpc"
	"log"
	"net"
	"net/http"
	grpc_handler "therealbroker/api/proto/handler"
	proto "therealbroker/api/proto/src"
	"therealbroker/internal/broker"
	broker2 "therealbroker/pkg/broker"
	"therealbroker/pkg/database"
	"time"
)

// Main requirements:
// 1. All tests should be passed
// 2. Your logs should be accessible in Graylog
// 3. Basic prometheus metrics ( latency, throughput, etc. ) should be implemented
// 	  for every base functionality ( publish, subscribe etc. )

func RunPromtheusServer(port string) {
	http.Handle("/metrics", promhttp.Handler())
	if err := http.ListenAndServe(fmt.Sprintf(":%s", port), nil); err != nil {
		log.Fatalf("promtheus server faild to serve on port %s, error : %v", port, err)
	}
}

func RunBrokerGrpcServer(port string, m broker2.Broker) {
	lis, err := net.Listen("tcp", fmt.Sprintf(":%s", port))
	if err != nil {
		log.Fatalf("failed to listen of port %s, error : %v", port, err)
	}

	s := grpc_handler.BrokerGrpcServer{M: m}

	grpcServer := grpc.NewServer()
	proto.RegisterBrokerServer(grpcServer, &s)

	log.Printf("server listening at %v", lis.Addr())
	if err := grpcServer.Serve(lis); err != nil {
		log.Fatalf("grpc server faild to serve on port %s, error : %v", port, err)
	}
}

func main() { // port haro bayad az enviorment variable gereft baraye dockeri kardan
	fmt.Println("Hello!")

	// running prometheus server
	prometheus.MustRegister(grpc_handler.PublishMethodCounter)
	prometheus.MustRegister(grpc_handler.SubscribeMethodCounter)
	prometheus.MustRegister(grpc_handler.FetchMethodCounter)
	prometheus.MustRegister(grpc_handler.PublishMethodDuration)
	prometheus.MustRegister(grpc_handler.SubscribeMethodDuration)
	prometheus.MustRegister(grpc_handler.FetchMethodDuration)
	prometheus.MustRegister(grpc_handler.TotalActiveSubscribers)
	go RunPromtheusServer("8000")

	//create the database and module and running the broker grpc server

	//db, err := database.NewPostgresDB(
	//	"postgresql://admin:admin@localhost:5432/broker?sslmode=disable",
	//	0,
	//	10000,
	//	time.Millisecond*100,
	//)

	idGenerator, err := database.NewSnowFlakeIDGenerator(1) // node id must be unique for each pod
	if err != nil {
		log.Fatal(err)
	}

	//idGenerator, err := database.NewRedisIDGenerator("redis://:@localhost:6379/0")
	//if err != nil {
	//	log.Fatal(err)
	//}

	db, err := database.NewCassandraDB(
		[]string{"localhost:9042"},
		idGenerator,
		10000,
		time.Millisecond*100,
	)
	if err != nil {
		log.Fatal(err)
	}

	m := broker.NewModule(db)
	RunBrokerGrpcServer("8080", m)

	//
	//
	//
	//
	//
	//}
	//go func() {
	//	id, err := db.InsertMessage("test", broker2.Message{Body: "texttttttttt", ExpirationTime: time.Now()})
	//	if err != nil {
	//		log.Println(err)
	//	}
	//	log.Println("id", id)
	//}()
	//time.Sleep(time.Second)
	//db.SendBatch()
	//time.Sleep(time.Second * 10)

	//msg, err := db.GetMessageBySubjectAndID("test", id)
	//if err != nil {
	//	log.Println(err)
	//}
	//println(msg.Body, msg.ID, msg.ExpirationTime.Format("2006/01/02 15:04"))
	//fmt.Println(time.Now().Format("2006/01/02 15:04"))
}
