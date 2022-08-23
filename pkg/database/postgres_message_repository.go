package database

import (
	"context"
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
}

func NewPostgresDB(connStr string, maxConn int32, batchSize int, batchInsertTimeLimit time.Duration) (*postgres, error) {
	// errors can be more specific (masalan baraye har err ye err jadid sakht ke moshkel + err ro bargardone....)
	config, err := pgxpool.ParseConfig(connStr)
	if err != nil {
		return nil, err
	}
	if maxConn != 0 {
		config.MaxConns = maxConn
	}
	pool, err := pgxpool.ConnectConfig(context.Background(), config)
	if err != nil {
		return nil, err
	}

	pg := postgres{
		pool:         pool,
		batchArray:   make([][]interface{}, batchSize),
		batchCounter: batchSize,
	}

	err = pg.CreateMessageTableIfNotExists()
	if err != nil {
		return nil, err
	}
	pg.idCounter, err = pg.GetMaxIdFromDB()
	if err != nil {
		return nil, err
	}
	ch, err := pg.CreateBatchInsertSchedule(batchInsertTimeLimit)
	if err != nil {
		return nil, err
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
	id := atomic.AddInt64(&pg.idCounter, 1)
	row := []interface{}{id, subject, message.Body, message.ExpirationTime}
	pg.batchLock.Lock()
	defer pg.batchLock.Unlock()
	pg.batchCounter -= 1
	pg.batchArray[pg.batchCounter] = row
	if pg.batchCounter == 0 {
		if _, err := pg.BatchInsertMessage(); err != nil {
			log.Println("couldn't do batch inserting : ", err)
			// adding the error for not inserting all the data
			return 0, err
		}
		pg.batchCounter = len(pg.batchArray)
		pg.batchInserted <- true
	}

	return id, nil
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

func (pg *postgres) BatchInsertMessage() (int64, error) {
	copyCount, err := pg.pool.CopyFrom(
		context.Background(),
		pgx.Identifier{"messages"},
		[]string{"id", "subject", "body", "expiration_time"},
		pgx.CopyFromRows(pg.batchArray[pg.batchCounter:]))
	return copyCount, err
}

func (pg *postgres) CreateMessageTableIfNotExists() error {
	_, err := pg.pool.Exec(context.Background(),
		`create table if not exists public.messages(
		id serial,
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
		"select (max(id), 0) from messages")
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
			if _, err := pg.BatchInsertMessage(); err != nil {
				log.Println("couldn't do batch inserting : ", err)
			}
			pg.batchCounter = len(pg.batchArray)
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
