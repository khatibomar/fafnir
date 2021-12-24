package fafnir

import (
	"fmt"
	"sync"
)

var (
	ErrEmptyQueue    = fmt.Errorf("Queue is empty")
	ErrQueueNotFound = fmt.Errorf("Queue doesn't exist")
)

type QueueRepository interface {
	Fetch() ([]*Queue, error)
	Create(string) error
	Delete(string) error
	Get(string, chan error) (*Queue, error)
}

type Queue struct {
	Name          string
	Entries       []Entry
	FailedEntries []Entry
	mu            sync.Mutex
}

func NewQueue(name string) *Queue {
	return &Queue{
		Name: name,
	}
}

func (q *Queue) EnQueue(e Entry) error {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.Entries = append(q.Entries, e)
	return nil
}

func (q *Queue) DeQueue() (Entry, error) {
	q.mu.Lock()
	defer q.mu.Unlock()
	if len(q.Entries) == 0 {
		return Entry{}, ErrEmptyQueue
	}
	e := q.Entries[0]
	q.Entries = q.Entries[1:]
	return e, nil
}

func (q *Queue) EnQueueFail(e Entry) error {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.FailedEntries = append(q.FailedEntries, e)
	return nil
}

func (q *Queue) DeQueueFail() (Entry, error) {
	q.mu.Lock()
	defer q.mu.Unlock()
	if len(q.FailedEntries) == 0 {
		return Entry{}, ErrEmptyQueue
	}
	e := q.FailedEntries[0]
	q.FailedEntries = q.FailedEntries[1:]
	return e, nil
}
