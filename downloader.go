package fafnir

import (
	"fmt"
	"os"
	"path"

	"github.com/cavaliercoder/grab"
)

type Fafnir struct {
	Queues []*Queue
	Cfg    *Config
}

func New(cfg *Config) *Fafnir {
	queues := []*Queue{}
	return &Fafnir{
		Queues: queues,
		Cfg:    cfg,
	}
}

func (f *Fafnir) Add(queueName, url, path, name string) {
	var e Entry
	e.DwnDir = path
	e.Filename = name
	e.Url = url
	for _, q := range f.Queues {
		if q.Name == queueName {
			q.EnQueue(e)
			break
		}
	}
	q := NewQueue(queueName)
	q.EnQueue(e)
	f.Queues = append(f.Queues, q)
}

func (f *Fafnir) AddQueue(queue *Queue) {
	for _, q := range f.Queues {
		if q.Name == queue.Name {
			q.Entries = append(q.Entries, queue.Entries...)
			break
		}
	}
	f.Queues = append(f.Queues, queue)
}

func (f *Fafnir) StartQueueDownload(queueName string) error {
	var que *Queue
	for _, q := range f.Queues {
		if q.Name == queueName {
			que = q
			break
		}
	}
	if que == nil {
		return ErrQueueNotFound
	} else if len(que.Entries) == 0 {
		return ErrEmptyQueue
	} else {
		jobsChan := make(chan Entry, len(que.Entries))
		errChan := make(chan error, len(que.Entries))

		for w := 1; w <= f.Cfg.MaxConcurrentDownloads; w++ {
			go f.Download(w, jobsChan, errChan)
		}

		for _, j := range que.Entries {
			jobsChan <- j
		}
		close(jobsChan)

		for i := 0; i < len(que.Entries); i++ {
			err := <-errChan
			fmt.Println(err)
		}
	}
	return nil
}

func (f *Fafnir) Download(id int, jobsChan <-chan Entry, errChan chan<- error) {
	for j := range jobsChan {
		err := os.MkdirAll(j.DwnDir, 0777)
		if err != nil {
			errChan <- err
			return
		}
		resp, err := grab.Get(path.Join(j.DwnDir, j.Filename), j.Url)
		if err != nil {
			errChan <- err
			return
		}
		if resp.Err() != nil {
			errChan <- resp.Err()
			return
		}
		errChan <- nil
	}
}
