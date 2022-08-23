package database

import (
	"sync"
	"therealbroker/pkg/broker"
)

type inMemorySubject struct {
	messages  map[int64]broker.Message
	lock      sync.RWMutex
	idCounter int64
}

type inMemory struct {
	subjects        map[string]*inMemorySubject
	allSubjectsLock sync.RWMutex
	//idCounter       int64
	//idCounterLock   sync.Mutex
}

func NewInMemoryDB() *inMemory {
	return &inMemory{
		subjects: make(map[string]*inMemorySubject),
	}
}

func (db *inMemory) GetMessageBySubjectAndID(subject string, id int64) (*broker.Message, error) {
	db.allSubjectsLock.RLock()
	inMemSubject, ok := db.subjects[subject]
	db.allSubjectsLock.RUnlock()
	if !ok {
		return &broker.Message{}, broker.ErrInvalidSubject
	}
	inMemSubject.lock.RLock()
	message, ok := inMemSubject.messages[id]
	inMemSubject.lock.RUnlock()
	if !ok {
		return &broker.Message{}, broker.ErrInvalidID
	}

	return &message, nil
}

func (db *inMemory) InsertMessage(subject string, message broker.Message) (int64, error) {
	db.allSubjectsLock.RLock()
	inMemSubject, ok := db.subjects[subject]
	db.allSubjectsLock.RUnlock()

	if !ok {
		db.allSubjectsLock.Lock()
		inMemSubject = &inMemorySubject{messages: make(map[int64]broker.Message), idCounter: 1}
		db.subjects[subject] = inMemSubject
		db.allSubjectsLock.Unlock()
	}

	inMemSubject.lock.Lock()
	defer inMemSubject.lock.Unlock()
	id, err := CreateID(inMemSubject, message.ID)
	if err != nil {
		return 0, err
	}
	inMemSubject.messages[id] = message
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

func CreateID(inMemSubject *inMemorySubject, messageID int64) (int64, error) {

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
