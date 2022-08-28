package grpc_handler

import (
	"context"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"log"
	"therealbroker/api/proto/src"
	"therealbroker/pkg/broker"
	"time"
)

// define metrics
var (
	ActiveSubscribers = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "broker_active_subscribers",
		Help: "number of active subscribers in broker",
	})
	MethodCalls = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "broker_method_count",
		Help: "number of method calls in broker",
	}, []string{"method"})

	MethodError = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "broker_method_error_count",
		Help: "counter error of each method",
	}, []string{"method"})
	MethodDuration = promauto.NewSummaryVec(prometheus.SummaryOpts{
		Name: "broker_method_duration",
		Help: "calculating the latency of grpc calls",
		Objectives: map[float64]float64{
			0.5:  0.05,
			0.9:  0.01,
			0.99: 0.001,
		},
	}, []string{"method"})
)

type BrokerGrpcServer struct {
	M broker.Broker
	proto.UnimplementedBrokerServer
}

func (s *BrokerGrpcServer) Publish(ctx context.Context, request *proto.PublishRequest) (*proto.PublishResponse, error) {
	timer := prometheus.NewTimer(MethodDuration.WithLabelValues("Publish"))
	defer timer.ObserveDuration()
	defer MethodCalls.WithLabelValues("Publish").Inc()

	msg := broker.Message{Body: string(request.Body), Expiration: time.Second * time.Duration(request.ExpirationSeconds)}
	id, err := s.M.Publish(ctx, request.Subject, msg)
	if err != nil {
		log.Println("couldn't publish the message : ", err)
		MethodError.WithLabelValues("Publish").Inc()
		return nil, err
	}

	return &proto.PublishResponse{Id: int64(id)}, nil
}

func (s *BrokerGrpcServer) Subscribe(request *proto.SubscribeRequest, stream proto.Broker_SubscribeServer) error {
	defer MethodCalls.WithLabelValues("Subscribe").Inc()
	timer := prometheus.NewTimer(MethodDuration.WithLabelValues("Subscribe"))

	channel, err := s.M.Subscribe(context.Background(), request.Subject)
	if err != nil {
		log.Println("couldn't subscribe to the subject : ", err)
		MethodError.WithLabelValues("Subscribe").Inc()
		return err
	}

	ActiveSubscribers.Inc()
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
	defer MethodCalls.WithLabelValues("Fetch").Inc()
	timer := prometheus.NewTimer(MethodDuration.WithLabelValues("Fetch"))
	defer timer.ObserveDuration()

	msg, err := s.M.Fetch(ctx, request.Subject, request.Id)
	if err != nil {
		log.Println("couldn't fetch the message : ", err)
		MethodError.WithLabelValues("Fetch").Inc()
		return nil, err
	}
	return &proto.MessageResponse{Body: []byte(msg.Body)}, nil
}
