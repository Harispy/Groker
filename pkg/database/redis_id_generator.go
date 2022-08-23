package database

import (
	"context"
	"github.com/go-redis/redis/v8"
	"log"
)

type redisIDGenerator struct {
	client *redis.Client
}

func NewRedisIDGenerator(connStr string) (*redisIDGenerator, error) {
	// connStr : "redis://<user>:<pass>@localhost:6379/<db>"
	opt, err := redis.ParseURL(connStr)
	if err != nil {
		panic(err)
	}
	redisClient := redis.NewClient(opt)

	redisClient.Set(context.Background(), "id", 0, -1)

	return &redisIDGenerator{client: redisClient}, err
}

func (rdb *redisIDGenerator) Generate() (int64, error) {
	id, err := rdb.client.Incr(context.Background(), "id").Uint64()
	if err != nil {
		log.Fatal(err)
	}
	return int64(id), nil
}
