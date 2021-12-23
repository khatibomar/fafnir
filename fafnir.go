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
	MaxConcurrentDownloads uint
	// MaxFailError is the maximum number that an entry can fail
	// after failing N times it will not make it will not be re-queued
	MaxFailError uint
	Repo         Repository
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
		jobsChan := make(chan Entry, f.Cfg.MaxConcurrentDownloads)

		wg := f.Cfg.WaitGroup
		if wg == nil {
			wg = &sync.WaitGroup{}
		}
		for w := 1; w <= int(f.Cfg.MaxConcurrentDownloads); w++ {
			go f.download(ctx, wg, que, jobsChan)
		}

		for {
			if len(que.Entries) == 0 {
				break
			}
			j, err := que.DeQueue()
			if err != nil {
				continue
			}
			if j.ExtraData.FailCount >= int(f.Cfg.MaxFailError) {
				err := que.EnQueueFail(j)
				if err != nil {
					f.Cfg.ErrChan <- err
				}
				continue
			}
			jobsChan <- j
			// get all entries in failed entries that
			// didn't yet reach max allowed failing
			// count
			for _, fe := range que.FailedEntries {
				_, err = que.DeQueueFail()
				if err != nil {
					f.Cfg.ErrChan <- err
					continue
				}
				if j.ExtraData.FailCount < int(f.Cfg.MaxFailError) {
					que.EnQueue(fe)
				} else {
					que.EnQueueFail(fe)
				}
			}
		}
		close(jobsChan)
	}
	return nil
}

func (f *Fafnir) download(ctx context.Context, wg *sync.WaitGroup, queue *Queue, jobsChan <-chan Entry) {
	client := grab.NewClient()
	for e := range jobsChan {
		err := os.MkdirAll(e.DwnDir, 0777)
		if err != nil {
			e.ExtraData.Err = err
			f.Cfg.Repo.Update(e)
			f.Cfg.ErrChan <- err
			continue
		}
		req, err := grab.NewRequest(path.Join(e.DwnDir, e.Filename), e.Url)
		if err != nil {
			e.ExtraData.Err = err
			f.Cfg.Repo.Update(e)
			f.Cfg.ErrChan <- err
			continue
		}
		resp := client.Do(req)

		// TODO(khatibomar): this is ugly unacceptable
		// need to take a look at how to make it better
		// with less repeatable code
		wg.Add(1)
		defer wg.Done()
		t := time.NewTicker(500 * time.Millisecond)
		defer t.Stop()
		func(e Entry) {
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
						e.ExtraData.FailCount++
						e.ExtraData.Err = dlerr
						select {
						case f.Cfg.ErrChan <- err:
						default:
						}
						queue.EnQueueFail(e)
					}
					f.Cfg.Repo.Update(e)
					return
				case <-ctx.Done():
					e.ExtraData.FailCount++
					err := resp.Cancel()
					if err != nil {
						f.Cfg.ErrChan <- err
					}
					queue.EnQueueFail(e)
					f.Cfg.Repo.Update(e)
					return
				}
			}
		}(e)
	}
}
