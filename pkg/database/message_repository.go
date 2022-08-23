package database

import "therealbroker/pkg/broker"

// MessageRepository : better to create worker pool in db side so the preformance goes up
// and add batch updating
type MessageRepository interface {
	GetMessageBySubjectAndID(subject string, id int64) (*broker.Message, error)
	InsertMessage(subject string, message *broker.Message) (int64, error)
	//GetSubjects() ([]string, error)
}

type MessageIDGenerator interface {
	Generate() (int64, error)
}
