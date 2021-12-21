package fafnir

import (
	"context"
	"os"
	"path"
	"sync"
	"time"

	"github.com/cavaliercoder/grab"
)

type Repository interface {
	QueueRepository
	EntryRepository
}

type Config struct {
	// ErrCh!=nil means errors during download are sent to
	// this channel for the client to consume.
	ErrChan                chan error
	MaxConcurrentDownloads int
	Repo                   Repository
	// WaitGroup!=nil will update the wait group as goroutines
	// are started and finished.
	WaitGroup *sync.WaitGroup
}

type Fafnir struct {
	Cfg *Config
}

func New(cfg *Config) (*Fafnir, error) {
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

func (f *Fafnir) StartQueueDownload(queueName string) error {
	return f.StartQueueDownloadWithCtx(context.Background(), queueName)
}

func (f *Fafnir) StartQueueDownloadWithCtx(ctx context.Context, queueName string) error {
	que, err := f.Cfg.Repo.Get(queueName)
	if err != nil {
		return err
	}
	if que == nil {
		return ErrQueueNotFound
	} else if len(que.Entries) == 0 {
		return ErrEmptyQueue
	} else {
		numJobs := len(que.Entries)
		jobsChan := make(chan Entry, numJobs)
		doneChan := make(chan bool, numJobs)

		wg := f.Cfg.WaitGroup
		if wg == nil {
			wg = &sync.WaitGroup{}
		}
		for w := 1; w <= f.Cfg.MaxConcurrentDownloads; w++ {
			go f.download(ctx, wg, jobsChan, doneChan)
		}

		for _, j := range que.Entries {
			jobsChan <- j
		}
		close(jobsChan)

		for i := 0; i < numJobs; i++ {
			<-doneChan
		}
	}
	return nil
}

func (f *Fafnir) download(ctx context.Context, wg *sync.WaitGroup, jobsChan <-chan Entry, doneChan chan<- bool) {
	for j := range jobsChan {
		err := os.MkdirAll(j.DwnDir, 0777)
		if err != nil {
			j.ExtraData.Err = err
			f.Cfg.Repo.Update(j)
			return
		}
		client := grab.NewClient()
		req, err := grab.NewRequest(path.Join(j.DwnDir, j.Filename), j.Url)
		if err != nil {
			j.ExtraData.Err = err
			f.Cfg.Repo.Update(j)
			return
		}
		resp := client.Do(req)

		// TODO(khatibomar): this is ugly unacceptable
		// need to take a look at how to make it better
		// with less repeatable code
		wg.Add(1)
		go func(r *grab.Response, e Entry) {
			defer wg.Done()
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
					dlerr := resp.Err()
					if dlerr != nil {
						e.ExtraData.Err = dlerr
						select {
						case f.Cfg.ErrChan <- err:
						default:
						}
					}
					f.Cfg.Repo.Update(e)
					doneChan <- true
					return
				case <-ctx.Done():
					err := resp.Cancel()
					if err != nil {
						f.Cfg.ErrChan <- err
					}
					return
				}
			}
		}(resp, j)
	}
}
