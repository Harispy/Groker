package broker

import (
	"context"
	"log"
	"sync"
	"therealbroker/pkg/broker"
	"therealbroker/pkg/database"
)

type Module struct {
	// TODO: Add required fields
	Closed       bool
	Subjects     map[string][]chan broker.Message
	SubjectsLock sync.RWMutex
	// can cache the messages it gets before
	Database broker.DataBase
}

func NewModule() broker.Broker {
	db := database.NewInMemoryDB() // temp db (persist one should pass into func args)
	subjects, err := CreateSubjectsMapFromDB(db)
	if err != nil {
		log.Fatalf("cannot get the subjects from the database: %s", err)
	}
	return &Module{
		Closed:   false,
		Subjects: subjects,
		Database: db,
	}
}

func (m *Module) Close() error {
	if m.Closed {
		return broker.ErrUnavailable
	}
	m.Closed = true
	return nil
	// do some other stuff here (for complete the closing) like send close message in channels
}

func (m *Module) Publish(ctx context.Context, subject string, msg broker.Message) (int, error) {
	if m.Closed {
		return 0, broker.ErrUnavailable
	}

	id, err := m.Database.InsertMessage(subject, msg)
	if err != nil {
		return 0, err
	}

	m.SubjectsLock.RLock()
	defer m.SubjectsLock.RUnlock()

	if subscribes, ok := m.Subjects[subject]; ok {
		for _, ch := range subscribes {
			ch <- msg
		}
	}
	return id, nil
}

func (m *Module) Subscribe(ctx context.Context, subject string) (<-chan broker.Message, error) {
	if m.Closed {
		return nil, broker.ErrUnavailable
	}
	m.SubjectsLock.Lock()
	defer m.SubjectsLock.Unlock()
	if _, ok := m.Subjects[subject]; !ok {
		m.Subjects[subject] = make([]chan broker.Message, 0)
	}
	channel := make(chan broker.Message, 100000)
	m.Subjects[subject] = append(m.Subjects[subject], channel)
	return channel, nil
}

func (m *Module) Fetch(ctx context.Context, subject string, id int) (broker.Message, error) {
	if m.Closed {
		return broker.Message{}, broker.ErrUnavailable
	}
	message, err := m.Database.GetMessageBySubjectAndID(subject, id)
	if err != nil {
		return broker.Message{}, err
	}
	return message, nil
}

func CreateSubjectsMapFromDB(db broker.DataBase) (map[string][]chan broker.Message, error) {
	subjs, err := db.GetSubjects()
	if err != nil {
		return nil, err
	}
	subjects := make(map[string][]chan broker.Message, len(subjs))
	for _, subj := range subjs {
		subjects[subj] = make([]chan broker.Message, 0)
	}
	return subjects, nil
}
