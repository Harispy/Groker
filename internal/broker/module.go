package broker

import (
	"context"
	"sync"
	"therealbroker/pkg/broker"
	"therealbroker/pkg/database"
	"time"
)

type Module struct {
	// TODO: Add required fields
	Closed       bool
	CloseLock    sync.RWMutex
	Subjects     map[string][]chan broker.Message
	SubjectsLock sync.RWMutex
	// can cache the messages it gets before
	Database database.MessageRepository
}

func NewModule(db database.MessageRepository) broker.Broker {
	return &Module{
		Closed:   false,
		Subjects: make(map[string][]chan broker.Message),
		Database: db,
	}
}

func (m *Module) Close() error {
	m.CloseLock.Lock()
	defer m.CloseLock.Unlock()
	if m.Closed {
		return broker.ErrUnavailable
	}

	m.Closed = true
	// closing all open channels
	for _, channels := range m.Subjects {
		for _, channel := range channels {
			close(channel)
		}
	}
	return nil
}

func (m *Module) Publish(ctx context.Context, subject string, msg broker.Message) (int64, error) {
	m.CloseLock.RLock()
	defer m.CloseLock.RUnlock()
	if m.Closed {
		return 0, broker.ErrUnavailable
	}

	var id int64 = 0
	var err error
	if msg.Expiration != 0 {
		msg.ExpirationTime = time.Now().Add(msg.Expiration)
		id, err = m.Database.InsertMessage(subject, &msg)
		if err != nil {
			return 0, err
		}
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
	m.CloseLock.RLock()
	defer m.CloseLock.RUnlock()
	if m.Closed {
		return nil, broker.ErrUnavailable
	}

	m.SubjectsLock.Lock()
	defer m.SubjectsLock.Unlock()
	if _, ok := m.Subjects[subject]; !ok {
		m.Subjects[subject] = make([]chan broker.Message, 0)
	}
	channel := make(chan broker.Message, 10000)
	m.Subjects[subject] = append(m.Subjects[subject], channel)
	return channel, nil
}

func (m *Module) Fetch(ctx context.Context, subject string, id int64) (*broker.Message, error) {
	m.CloseLock.RLock()
	defer m.CloseLock.RUnlock()
	if m.Closed {
		return &broker.Message{}, broker.ErrUnavailable
	}

	message, err := m.Database.GetMessageBySubjectAndID(subject, id)
	if err != nil {
		return &broker.Message{}, err
	}
	if time.Now().After(message.ExpirationTime) {
		return &broker.Message{}, broker.ErrExpiredID
	}
	return message, nil
}

//func CreateSubjectsMapFromDB(db database.MessageRepository) (map[string][]chan broker.Message, error) {
//	subjs, err := db.GetSubjects()
//	if err != nil {
//		return nil, err
//	}
//	subjects := make(map[string][]chan broker.Message, len(subjs))
//	for _, subj := range subjs {
//		subjects[subj] = make([]chan broker.Message, 0)
//	}
//	return subjects, nil
//}
