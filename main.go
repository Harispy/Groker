package main

import (
	"fmt"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"google.golang.org/grpc"
	"log"
	"net"
	"net/http"
	"os"
	"strconv"
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

func SetDefaultEnvVars() error {
	if os.Getenv("NODE_ID") == "" {
		err := os.Setenv("NODE_ID", "1")
		if err != nil {
			return err
		}
	}
	if os.Getenv("BROKER_DATABASE") == "" {
		err := os.Setenv("BROKER_DATABASE", "cassandra")
		if err != nil {
			return err
		}
	}
	if os.Getenv("MESSAGE_ID_GENERATOR") == "" {
		err := os.Setenv("MESSAGE_ID_GENERATOR", "snowflake")
		if err != nil {
			return err
		}
	}
	if os.Getenv("BROKER_DATABASE_HOST") == "" {
		err := os.Setenv("BROKER_DATABASE_HOST", "cassandra")
		if err != nil {
			return err
		}
	}
	if os.Getenv("REDIS_ID_GENERATOR_HOST") == "" {
		err := os.Setenv("REDIS_ID_GENERATOR_HOST", "redis")
		if err != nil {
			return err
		}
	}
	return nil
}

func main() {
	// port haro bayad az enviorment variable gereft baraye dockeri kardan
	// ham chenin host name ha
	fmt.Println("Hello! v2.0.0")

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

	// ENV VARs :
	// 	BROKER_DATABASE
	// 	NODE_ID
	// 	MESSAGE_ID_GENERATOR
	// 	BROKER_DATABASE_HOST

	var (
		inodeID     int
		idGenerator database.MessageIDGenerator
		err         error
		db          database.MessageRepository
	)

	// set default ENV VARs :
	err = SetDefaultEnvVars()
	if err != nil {
		log.Fatal("error while setting default value for env variables")
	}

	switch os.Getenv("NODE_ID") {
	default:
		inodeID, err = strconv.Atoi(os.Getenv("NODE_ID"))
		if err != nil {
			log.Fatal("node id should be integer")
		}
		log.Println("node id set to:", os.Getenv("NODE_ID"))
	}

	// create message id generator
	err = nil
	switch os.Getenv("MESSAGE_ID_GENERATOR") {
	case "snowflake":
		idGenerator, err = database.NewSnowFlakeIDGenerator(int64(inodeID)) // node id must be unique for each pod
	case "redis":
		idGenerator, err = database.NewRedisIDGenerator(
			fmt.Sprintf("redis://:@%s:6379/0", os.Getenv("REDIS_ID_GENERATOR_HOST")),
		)
	default:
		log.Fatal("invalid id generator!")
	}
	if err != nil {
		log.Fatal(err)
	} else {
		log.Printf("%s ID Generator created\n", os.Getenv("MESSAGE_ID_GENERATOR"))
	}

	// create broker database
	err = nil
	switch os.Getenv("BROKER_DATABASE") {

	case "cassandra":
		db, err = database.NewCassandraDB(
			[]string{fmt.Sprintf("%s:9042", os.Getenv("BROKER_DATABASE_HOST"))},
			idGenerator,
			10000,
			time.Millisecond*100,
		)

	case "postgres":
		db, err = database.NewPostgresDB(
			fmt.Sprintf("postgres://admin:admin@%s:5432/broker?sslmode=disable", os.Getenv("BROKER_DATABASE_HOST")),
			0,
			10000,
			time.Millisecond*100,
		)

	case "inMemory":
		db = database.NewInMemoryDB()

	default:
		log.Fatal("invalid broker database")
	}
	if err != nil {
		log.Fatal(err)
	} else {
		log.Printf("%s database created successfully\n", os.Getenv("BROKER_DATABASE"))
	}

	// create the broker module
	m := broker.NewModule(db)
	RunBrokerGrpcServer("8080", m)

}
