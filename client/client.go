package main

import (
	"context"
	"fmt"
	"google.golang.org/grpc"
	"log"
	"sync"
	proto "therealbroker/api/proto/src"
)

func Publish(client proto.BrokerClient, message string, subject string) {
	_, err := client.Publish(context.Background(), &proto.PublishRequest{
		Subject:           subject,
		Body:              []byte(message),
		ExpirationSeconds: 2000,
	})
	if err != nil {
		log.Println("Error publishing message: ", err)
		return
	}
	//if res.Id%1 == 0 {
	//	log.Println(res.Id)
	//}
}

func Subscribe(client proto.BrokerClient, subject string) {
	_, err := client.Subscribe(context.Background(), &proto.SubscribeRequest{
		Subject: subject,
	})
	if err != nil {
		log.Println("Error Subscribing message: ", err)
		return
	}
}

func main() {

	var wg2 sync.WaitGroup
	for k := 0; k < 6; k++ {
		wg2.Add(1)
		go func(wg2 *sync.WaitGroup) {
			defer wg2.Done()
			conn, err := grpc.Dial("192.168.70.191:30008", grpc.WithInsecure()) //  192.168.70.191:30008
			if err != nil {
				log.Println("Error connecting to broker: ", err)
				return
			}
			defer conn.Close()

			client := proto.NewBrokerClient(conn)

			var wg sync.WaitGroup
			for j := 0; j < 6666; j++ { // 20000
				wg.Add(1)
				go func(wg *sync.WaitGroup, j int) {
					for i := 0; i < 1000000; i++ { // 100000
						Publish(client, fmt.Sprintf("hi my dear friends!! %v %v", j, i), "test")
					}
					wg.Done()
				}(&wg, j)
			}
			wg.Wait()
		}(&wg2)

	}
	wg2.Wait()

}
