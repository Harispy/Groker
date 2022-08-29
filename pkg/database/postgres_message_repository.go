package database

import (
	"context"
	"fmt"
	"github.com/jackc/pgx/v4"
	"github.com/jackc/pgx/v4/pgxpool"
	"log"
	"sync"
	"sync/atomic"
	"therealbroker/pkg/broker"
	"time"
)

type postgres struct {
	pool          *pgxpool.Pool // can be replaced with a pool
	idCounter     int64
	batchArray    [][]interface{}
	batchCounter  int
	batchLock     sync.Mutex
	batchInserted chan<- bool
	channels      []chan<- error
}

func NewPostgresDB(connStr string, maxConn int32, batchSize int, batchInsertTimeLimit time.Duration) (*postgres, error) {
	// errors can be more specific (masalan baraye har err ye err jadid sakht ke moshkel + err ro bargardone....)
	var (
		pool *pgxpool.Pool
		err  error
	)
	config, err := pgxpool.ParseConfig(connStr)
	if err != nil {
		return nil, fmt.Errorf("error while parsing config : %w", err)
	}
	if maxConn != 0 {
		config.MaxConns = maxConn
	}
	for i := 1; i < 20; i++ {
		pool, err = pgxpool.ConnectConfig(context.Background(), config)
		if err == nil {
			break
		}
		time.Sleep(time.Second * time.Duration(i))
	}
	if err != nil {
		return nil, fmt.Errorf("error while connecting to postgres : %w", err)
	}

	pg := postgres{
		pool:         pool,
		batchArray:   make([][]interface{}, batchSize),
		batchCounter: batchSize,
		channels:     make([]chan<- error, 0, batchSize),
	}

	err = pg.CreateMessageTableIfNotExists()
	if err != nil {
		return nil, fmt.Errorf("error while creating table : %w", err)
	}
	pg.idCounter, err = pg.GetMaxIdFromDB()
	if err != nil {
		return nil, fmt.Errorf("error while getting max id from db : %w", err)
	}
	ch, err := pg.CreateBatchInsertSchedule(batchInsertTimeLimit)
	if err != nil {
		return nil, fmt.Errorf("error while creating batch insert schedule : %w", err)
	}
	pg.batchInserted = ch
	return &pg, nil
}

func (pg *postgres) GetMessageBySubjectAndID(subject string, id int64) (*broker.Message, error) {
	var message broker.Message
	row := pg.pool.QueryRow(context.Background(),
		"SELECT id, body, expiration_time FROM messages WHERE subject=$1 AND id=$2", subject, id)
	err := row.Scan(&message.ID, &message.Body, &message.ExpirationTime)
	if err == pgx.ErrNoRows {
		return &broker.Message{}, broker.ErrInvalidID
	} else if err != nil {
		return &broker.Message{}, err
	}

	return &message, nil
}

func (pg *postgres) InsertMessage(subject string, message *broker.Message) (int64, error) {
	message.ID = atomic.AddInt64(&pg.idCounter, 1)

	channel, err := pg.AddToBatch(subject, message)
	if err != nil {
		return 0, fmt.Errorf("error while adding to batch : %w", err)
	}
	return message.ID, <-channel
}

func (pg *postgres) SingleInsertMessage(subject string, message *broker.Message) (int64, error) {
	id := atomic.AddInt64(&pg.idCounter, 1)
	_, err := pg.pool.Exec(context.Background(),
		"INSERT INTO messages (id, subject, body, expiration_time) VALUES ($1, $2, $3, $4)",
		id, subject, message.Body, message.ExpirationTime)
	if err != nil {
		return 0, err
	}
	return id, nil
}

func (pg *postgres) AddToBatch(subject string, message *broker.Message) (<-chan error, error) {
	row := []interface{}{message.ID, subject, message.Body, message.ExpirationTime}
	pg.batchLock.Lock()
	defer pg.batchLock.Unlock()

	pg.batchCounter -= 1
	pg.batchArray[pg.batchCounter] = row
	channel := make(chan error, 1)
	pg.channels = append(pg.channels, channel)
	if pg.batchCounter == 0 {
		pg.BatchInsertMessage()
		pg.batchInserted <- true
	}
	return channel, nil
}

func (pg *postgres) BatchInsertMessage() (int64, error) {
	copyCount, err := pg.pool.CopyFrom(
		context.Background(),
		pgx.Identifier{"messages"},
		[]string{"id", "subject", "body", "expiration_time"},
		pgx.CopyFromRows(pg.batchArray[pg.batchCounter:]))
	pg.batchCounter = len(pg.batchArray)
	for _, channel := range pg.channels {
		channel <- err
	}
	pg.channels = make([]chan<- error, 0, len(pg.batchArray))
	if err != nil {
		log.Println("couldn't do batch inserting : ", err)
	}
	return copyCount, err
}

func (pg *postgres) CreateMessageTableIfNotExists() error {
	_, err := pg.pool.Exec(context.Background(),
		`create table if not exists public.messages(
		id bigserial,
    	subject text,
    	body text,
    	expiration_time timestamp,
    	constraint message_id
        	primary key (subject, id) );`)
	if err != nil {
		return err
	}
	return nil
}

func (pg *postgres) GetMaxIdFromDB() (int64, error) {
	var maxId int64
	row := pg.pool.QueryRow(context.Background(),
		"select COALESCE(max(id), 0) from messages")
	if err := row.Scan(&maxId); err != nil {
		return 0, err
	}
	return maxId, nil
}

func (pg *postgres) CreateBatchInsertSchedule(timePeriod time.Duration) (chan<- bool, error) {
	inserted := make(chan bool, 1000)
	go pg.BatchInsertSchedule(inserted, timePeriod)
	return inserted, nil
}

func (pg *postgres) BatchInsertSchedule(inserted <-chan bool, timePeriod time.Duration) {
	for {
		select {
		case <-inserted:
			break
		case <-time.After(timePeriod):
			pg.batchLock.Lock()
			if pg.batchCounter == len(pg.batchArray) {
				pg.batchLock.Unlock()
				break
			}
			pg.BatchInsertMessage()
			pg.batchLock.Unlock()
		}
	}
}

//func (pg *postgres) CreateConfigTableIfNotExists() error {
//	_, err := pg.pool.Exec(context.Background(),
//		`create table if not exists public.config(
//		"key" text,
//		"value" text);`)
//	if err != nil {
//		return err
//	}
//	return nil
//}
