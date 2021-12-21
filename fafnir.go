package fafnir

import (
	"os"
	"path"
	"time"

	"github.com/cavaliercoder/grab"
)

type Repository interface {
	QueueRepository
	EntryRepository
}

type FafnirConfig struct {
	MaxConcurrentDownloads int
	Repo                   Repository
}

func NewFafnirConfig(maxConcurrentDownloads int, repo Repository) *FafnirConfig {
	return &FafnirConfig{
		MaxConcurrentDownloads: maxConcurrentDownloads,
		Repo:                   repo,
	}
}

type Fafnir struct {
	Cfg *FafnirConfig
}

func NewFafnir(cfg *FafnirConfig) (*Fafnir, error) {
	return &Fafnir{
		Cfg: cfg,
	}, nil
}

func (f *Fafnir) Add(queueName, url, path, name string) error {
	var e Entry

	e.DwnDir = path
	e.Filename = name
	e.Url = url

	_, err := f.Cfg.Repo.Get(queueName)

	if err != nil {
		err := f.Cfg.Repo.Create(queueName)
		if err != nil {
			return err
		}
	}
	return f.Cfg.Repo.Add(queueName, e)
}

func (f *Fafnir) StartQueueDownload(queueName string, errChan chan<- error) {
	que, err := f.Cfg.Repo.Get(queueName)
	if err != nil {
		errChan <- err
		return
	}
	if que == nil {
		errChan <- ErrQueueNotFound
		return
	} else if len(que.Entries) == 0 {
		errChan <- ErrEmptyQueue
		return
	} else {
		jobsChan := make(chan Entry, len(que.Entries))
		errortoChan := make(chan error, len(que.Entries))

		for w := 1; w <= f.Cfg.MaxConcurrentDownloads; w++ {
			go f.Download(jobsChan, errortoChan)
		}

		for _, j := range que.Entries {
			jobsChan <- j
		}
		close(jobsChan)

		for i := 0; i < len(que.Entries); i++ {
			err = <-errortoChan
			errChan <- err
		}
		close(errortoChan)
	}
}

func (f *Fafnir) Download(jobsChan <-chan Entry, errChan chan<- error) {
	for j := range jobsChan {
		err := os.MkdirAll(j.DwnDir, 0777)
		if err != nil {
			errChan <- err
			return
		}
		client := grab.NewClient()
		req, err := grab.NewRequest(path.Join(j.DwnDir, j.Filename), j.Url)
		if err != nil {
			errChan <- err
			return
		}
		resp := client.Do(req)

		// TODO(khatibomar): this is ugly unacceptable
		// need to take a look at how to make it better
		// with less repeatable code
		go func(r *grab.Response, e Entry, errChan chan<- error) {
			t := time.NewTicker(500 * time.Millisecond)
			defer t.Stop()
			for {
				select {
				case <-t.C:
					e.ExtraData.BytesPerSecond = resp.BytesPerSecond()
					e.ExtraData.BytesTransfered = uint64(resp.BytesComplete())
					e.ExtraData.CanResume = resp.CanResume
					e.ExtraData.DidResume = resp.DidResume
					e.ExtraData.Duration = resp.Duration()
					e.ExtraData.ETA = resp.ETA()
					e.ExtraData.End = resp.End
					e.ExtraData.Progress = resp.Progress()
					e.ExtraData.Size = uint64(resp.Size)
					e.ExtraData.Start = resp.Start
					f.Cfg.Repo.Update(e)
				case <-resp.Done:
					// download is complete
					e.ExtraData.BytesPerSecond = resp.BytesPerSecond()
					e.ExtraData.BytesTransfered = uint64(resp.BytesComplete())
					e.ExtraData.CanResume = resp.CanResume
					e.ExtraData.DidResume = resp.DidResume
					e.ExtraData.Duration = resp.Duration()
					e.ExtraData.ETA = resp.ETA()
					e.ExtraData.End = resp.End
					e.ExtraData.Progress = resp.Progress()
					e.ExtraData.Size = uint64(resp.Size)
					e.ExtraData.Start = resp.Start
					e.ExtraData.Err = resp.Err()
					f.Cfg.Repo.Update(e)
					errChan <- nil
					return
				}
			}
		}(resp, j, errChan)
	}
}
