package database

import (
	"sync"
	"therealbroker/pkg/broker"
	"time"
)

type inMemoryMessage struct {
	message         broker.Message
	expiration_time time.Time
}

type inMemorySubject struct {
	messages  map[int]inMemoryMessage
	lock      sync.RWMutex
	idCounter int
}

type inMemory struct {
	subjects        map[string]*inMemorySubject
	allSubjectsLock sync.RWMutex
	//idCounter       int
	//idCounterLock   sync.Mutex
}

func NewInMemoryDB() *inMemory {
	return &inMemory{
		subjects: make(map[string]*inMemorySubject),
	}
}

func (db *inMemory) GetMessageBySubjectAndID(subject string, id int) (broker.Message, error) {
	db.allSubjectsLock.RLock()
	inMemSubject, ok := db.subjects[subject]
	db.allSubjectsLock.RUnlock()
	if !ok {
		return broker.Message{}, broker.ErrInvalidSubject
	}
	inMemSubject.lock.RLock()
	message, ok := inMemSubject.messages[id]
	inMemSubject.lock.RUnlock()
	if !ok {
		return broker.Message{}, broker.ErrInvalidID
	}
	if time.Now().After(message.expiration_time) {
		return broker.Message{}, broker.ErrExpiredID
	}
	return message.message, nil
}

func (db *inMemory) InsertMessage(subject string, message broker.Message) (int, error) {
	db.allSubjectsLock.RLock()
	inMemSubject, ok := db.subjects[subject]
	db.allSubjectsLock.RUnlock()

	if !ok {
		db.allSubjectsLock.Lock()
		inMemSubject = &inMemorySubject{messages: make(map[int]inMemoryMessage)}
		db.subjects[subject] = inMemSubject
		db.allSubjectsLock.Unlock()
	}

	inMemSubject.lock.Lock()
	defer inMemSubject.lock.Unlock()
	id, err := CreateID(inMemSubject, message.ID)
	if err != nil {
		return 0, err
	}
	inMemSubject.messages[id] = inMemoryMessage{message: message, expiration_time: time.Now().Add(message.Expiration)}
	return id, nil
}

func (db *inMemory) GetSubjects() ([]string, error) {
	// get the keys of db.messages (can be done in separate function)
	subjects := make([]string, len(db.subjects))
	i := 0
	for subject := range db.subjects {
		subjects[i] = subject
		i++
	}
	return subjects, nil
}

func CreateID(inMemSubject *inMemorySubject, messageID int) (int, error) {

	if messageID != 0 {
		if messageID < inMemSubject.idCounter {
			return 0, broker.ErrNotUnique
		}
		inMemSubject.idCounter = messageID + 1
		return messageID, nil
	}
	inMemSubject.idCounter++
	return inMemSubject.idCounter - 1, nil
}
