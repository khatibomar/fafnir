package repository

import (
	"errors"
	"sync"

	"codeberg.org/omarkhatib/fafnir"
)

var (
	ErrQueueNotFound     = errors.New("queue Not found")
	ErrQueueAlreadyExist = errors.New("queue Already Exist")
	ErrEntryNotFound     = errors.New("entry Not found")
)

type InMemoryRepo struct {
	mu      sync.Mutex
	Queues  []*fafnir.Queue
	Entries []fafnir.Entry
	EID     int
}

func NewInMemoryRepo() *InMemoryRepo {
	return &InMemoryRepo{
		Queues:  make([]*fafnir.Queue, 0),
		Entries: make([]fafnir.Entry, 0),
	}
}

func (repo *InMemoryRepo) Update(e fafnir.Entry) error {
	repo.mu.Lock()
	defer repo.mu.Unlock()
	if e.ID > repo.Entries[len(repo.Entries)-1].ID {
		return ErrEntryNotFound
	}
	repo.Entries[e.ID].ExtraData = e.ExtraData
	return nil
}

func (repo *InMemoryRepo) Add(queueName, link, dwnDir, filename, description string) error {
	repo.mu.Lock()
	defer repo.mu.Unlock()
	var e fafnir.Entry
	e.ID = repo.EID
	e.Filename = filename
	e.DwnDir = dwnDir
	e.Url = link
	e.Description = description
	for _, q := range repo.Queues {
		if queueName == q.Name {
			e.ID = repo.EID
			q.Entries = append(q.Entries, e)
			repo.Entries = append(repo.Entries, e)
			repo.EID++
			return nil
		}
	}
	return ErrQueueNotFound
}

func (repo *InMemoryRepo) Fetch() ([]*fafnir.Queue, error) {
	return nil, nil
}

func (repo *InMemoryRepo) Create(queueName string) error {
	repo.mu.Lock()
	defer repo.mu.Unlock()
	for _, q := range repo.Queues {
		if queueName == q.Name {
			return ErrQueueAlreadyExist
		}
	}
	repo.Queues = append(repo.Queues, fafnir.NewQueue(queueName))
	return nil
}

func (repo *InMemoryRepo) Delete(queueName string) error {
	repo.mu.Lock()
	defer repo.mu.Unlock()
	for index := range repo.Queues {
		if queueName == repo.Queues[index].Name {
			repo.Queues = append(repo.Queues[:index], repo.Queues[index+1:]...)
			return nil
		}
	}
	return ErrQueueNotFound
}

func (repo *InMemoryRepo) Get(queueName string, errChan chan error) (*fafnir.Queue, error) {
	repo.mu.Lock()
	defer repo.mu.Unlock()
	for _, q := range repo.Queues {
		if queueName == q.Name {
			return q, nil
		}
	}
	return nil, ErrQueueNotFound
}

func (repo *InMemoryRepo) Complete(e fafnir.Entry) error {
	return nil
}
