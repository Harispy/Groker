package main

import (
	"context"
	"github.com/go-redis/redis/v8"
	"log"
)

func main() {
	rdb := redis.NewClient(&redis.Options{
		Addr:     "localhost:6379",
		Password: "", // no password set
		DB:       0,  // use default DB
	})

	id, err := rdb.Incr(context.Background(), "id").Uint64()
	if err != nil {
		log.Fatal(err)
	}
	println(id)
}
