package grpc_handler

import (
	"context"
	"github.com/prometheus/client_golang/prometheus"
	"log"
	"therealbroker/api/proto/src"
	"therealbroker/pkg/broker"
	"time"
)

// define metrics
var PublishMethodCounter = prometheus.NewCounter(
	prometheus.CounterOpts{
		Name: "broker_publish_method_counter",
		Help: "number of publish method calls",
	})
var SubscribeMethodCounter = prometheus.NewCounter(
	prometheus.CounterOpts{
		Name: "broker_subscribe_method_counter",
		Help: "number of subscribe method calls",
	})
var FetchMethodCounter = prometheus.NewCounter(
	prometheus.CounterOpts{
		Name: "broker_fetch_method_counter",
		Help: "number of fetch method calls",
	})
var PublishMethodDuration = prometheus.NewSummary(
	prometheus.SummaryOpts{
		Name: "broker_publish_method_duration",
		Help: "the duration of executing public method",
		//Objectives: map[float64]float64{99: 0.05, 95: 0.05, 50: 0.05},
	})
var SubscribeMethodDuration = prometheus.NewSummary(
	prometheus.SummaryOpts{
		Name: "broker_subscribe_method_duration",
		Help: "the duration of executing subscribe method",
		//Objectives: map[float64]float64{99: 0.05, 95: 0.05, 50: 0.05},
	})
var FetchMethodDuration = prometheus.NewSummary(
	prometheus.SummaryOpts{
		Name: "broker_fetch_method_duration",
		Help: "the duration of executing fetch method",
		//Objectives: map[float64]float64{99: 0.05, 95: 0.05, 50: 0.05},
	})
var TotalActiveSubscribers = prometheus.NewGauge(
	prometheus.GaugeOpts{
		Name: "broker_active_subscribers",
		Help: "number of total active subscribers",
	})

type BrokerGrpcServer struct {
	M broker.Broker
	proto.UnimplementedBrokerServer
}

func (s *BrokerGrpcServer) Publish(ctx context.Context, request *proto.PublishRequest) (*proto.PublishResponse, error) {
	timer := prometheus.NewTimer(PublishMethodDuration)
	defer timer.ObserveDuration()
	defer PublishMethodCounter.Inc()

	msg := broker.Message{Body: string(request.Body), Expiration: time.Second * time.Duration(request.ExpirationSeconds)}
	id, err := s.M.Publish(ctx, request.Subject, msg)
	if err != nil {
		log.Println("couldn't publish the message : ", err)
		return nil, err
	}

	return &proto.PublishResponse{Id: int64(id)}, nil
}

func (s *BrokerGrpcServer) Subscribe(request *proto.SubscribeRequest, stream proto.Broker_SubscribeServer) error {
	SubscribeMethodCounter.Inc()
	timer := prometheus.NewTimer(SubscribeMethodDuration)

	channel, err := s.M.Subscribe(context.Background(), request.Subject)
	if err != nil {
		log.Println("couldn't subscribe to the subject : ", err)
		return err
	}

	TotalActiveSubscribers.Inc()
	timer.ObserveDuration()

	for {
		msg, more := <-channel
		if !more {
			break
		}
		if err := stream.Send(&proto.MessageResponse{Body: []byte(msg.Body)}); err != nil {
			log.Println("couldn't sent the message to the client")
		}
	}
	return nil
}

func (s *BrokerGrpcServer) Fetch(ctx context.Context, request *proto.FetchRequest) (*proto.MessageResponse, error) {
	FetchMethodCounter.Inc()
	timer := prometheus.NewTimer(FetchMethodDuration)
	defer timer.ObserveDuration()

	msg, err := s.M.Fetch(ctx, request.Subject, request.Id)
	if err != nil {
		log.Println("couldn't fetch the message : ", err)
		return nil, err
	}
	return &proto.MessageResponse{Body: []byte(msg.Body)}, nil
}
