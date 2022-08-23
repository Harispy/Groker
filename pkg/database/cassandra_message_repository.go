package database

import (
	"context"
	"fmt"
	"github.com/gocql/gocql"
	"log"
	"sync"
	"therealbroker/pkg/broker"
	"time"
)

type Batch struct {
	batch     *gocql.Batch
	batchLock sync.Mutex
	channels  []chan<- error
	inserted  chan<- bool
}

type cassandra struct {
	cluster              *gocql.ClusterConfig
	session              *gocql.Session
	batch                *Batch
	batchSize            int
	batchInsertTimeLimit time.Duration
	idGenerator          MessageIDGenerator
}

// can have a pool for batch inserting...
// make the input and output messages pointer to avoide copy data
// (actually impress performance (about 6%) and memory usage )

func NewCassandraDB(hosts []string, idGenerator MessageIDGenerator, batchSize int, batchInsertTimeLimit time.Duration) (*cassandra, error) {
	cluster := gocql.NewCluster(hosts...)
	//cluster.Keyspace = "broker"
	cluster.Consistency = gocql.Quorum
	//cluster.ProtoVersion = 4
	session, err := cluster.CreateSession()
	if err != nil {
		return nil, err
	}

	batch := session.NewBatch(gocql.LoggedBatch).WithContext(context.Background())
	batch.Entries = make([]gocql.BatchEntry, 0, batchSize)
	channels := make([]chan<- error, 0, batchSize)

	cs := cassandra{
		cluster:              cluster,
		session:              session,
		batch:                &Batch{batch: batch, channels: channels},
		batchSize:            batchSize,
		batchInsertTimeLimit: batchInsertTimeLimit,
		idGenerator:          idGenerator,
	}

	err = cs.CreateKeySpaceIfNotExists()
	if err != nil {
		return nil, err
	}
	err = cs.CreateMessageTableIfNotExists()
	if err != nil {
		return nil, err
	}
	err = cs.CreateBatchInsertSchedule(cs.batch)
	if err != nil {
		return nil, err
	}

	return &cs, nil
}

func (cs *cassandra) GetMessageBySubjectAndID(subject string, id int64) (*broker.Message, error) {
	var message broker.Message
	err := cs.session.Query(
		`SELECT id, body, expiration_time FROM broker.messages WHERE subject=? AND id=?`,
		subject, id,
	).WithContext(context.Background()).Scan(&message.ID, &message.Body, &message.ExpirationTime)

	if err == gocql.ErrNotFound {
		return &broker.Message{}, broker.ErrInvalidID
	} else if err != nil {
		return &broker.Message{}, err
	}

	return &message, nil
}

func (cs *cassandra) InsertMessage(subject string, message *broker.Message) (int64, error) {
	id, err := cs.idGenerator.Generate()
	if err != nil {
		return 0, fmt.Errorf("error while creating id : %w", err)
	}
	message.ID = id
	channel, err := cs.AddToBatch(subject, message)
	if err != nil {
		return 0, fmt.Errorf("error while adding to batch : %w", err)
	}
	return id, <-channel
}

func (cs *cassandra) SingleInsertMessage(subject string, message *broker.Message) (int64, error) {
	id, err := cs.idGenerator.Generate()
	if err != nil {
		return 0, fmt.Errorf("error while creating id : %w", err)
	}
	err = cs.session.Query(
		`INSERT INTO broker.messages (id, subject, body, expiration_time) VALUES (?, ?, ?, ?);`,
		id, subject, message.Body, message.ExpirationTime).Exec()

	if err != nil {
		return 0, fmt.Errorf("error while inserting to database : %w", err)
	}

	return id, nil
}

func (cs *cassandra) AddToBatch(subject string, message *broker.Message) (<-chan error, error) {
	// id should be specified in the message
	cs.batch.batchLock.Lock()
	defer cs.batch.batchLock.Unlock()
	cs.batch.batch.Entries = append(cs.batch.batch.Entries, gocql.BatchEntry{
		Stmt:       "INSERT INTO broker.messages (id, subject, body, expiration_time) VALUES (?, ?, ?, ?);",
		Args:       []interface{}{message.ID, subject, message.Body, message.ExpirationTime},
		Idempotent: true,
	})
	channel := make(chan error, 1)
	cs.batch.channels = append(cs.batch.channels, channel)

	if len(cs.batch.batch.Entries) == cs.batchSize {
		go cs.SendBatch()
	}
	return channel, nil
}

func (cs *cassandra) CreateKeySpaceIfNotExists() error {
	query := `CREATE KEYSPACE if not exists broker with replication = { 'class' : 'SimpleStrategy', 'replication_factor' : '1' };`
	if err := cs.session.Query(query).Exec(); err != nil {
		return err
	}
	return nil
}

func (cs *cassandra) CreateMessageTableIfNotExists() error {
	query := `create table if not exists broker.messages(
    	id bigint,
        subject text,
        body text,
        expiration_time timestamp,
        primary key (subject, id) );`
	if err := cs.session.Query(query).Exec(); err != nil {
		return err
	}
	return nil
}

//
//func (cs *cassandra) GetMaxIdFromDB() (int64, error) {
//	var maxId int64
//	err := cs.session.Query(`select max(id) from broker.messages`).WithContext(context.Background()).Scan(&maxId)
//	if err != nil {
//		return 0, fmt.Errorf("error while getting max id from database : %w", err)
//	}
//	return maxId, nil
//}

func (cs *cassandra) CreateBatchInsertSchedule(b *Batch) error {
	inserted := make(chan bool, 1000)
	go cs.BatchInsertSchedule(inserted)
	b.inserted = inserted
	return nil
}

func (cs *cassandra) BatchInsertSchedule(inserted <-chan bool) {
	for {
		select {
		case <-inserted:
			break
		case <-time.After(cs.batchInsertTimeLimit):
			if err := cs.SendBatch(); err != nil {
				log.Println("couldn't do batch inserting : ", err)
			}
		}
	}
}

func (cs *cassandra) SendBatch() error {
	cs.batch.batchLock.Lock()
	defer cs.batch.batchLock.Unlock()
	err := cs.session.ExecuteBatch(cs.batch.batch)
	// sending the result to all channels
	for _, channel := range cs.batch.channels {
		channel <- err
	}
	cs.batch.batch.Entries = make([]gocql.BatchEntry, 0, cs.batchSize)
	cs.batch.channels = make([]chan<- error, 0, cs.batchSize)
	return err
}
