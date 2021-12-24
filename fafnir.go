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
	// UpdateTimeMs specify the rate of which the info about the
	// entity that is currently downloading should be updated ,
	// this should be in millisecond (ms)
	UpdateTimeMs uint
	Repo         Repository
	// WaitGroup!=nil will update the wait group as goroutines
	// are started and finished.
	WaitGroup *sync.WaitGroup
}

type Fafnir struct {
	Cfg *Config
}

func New(cfg *Config) (*Fafnir, error) {
	if cfg.UpdateTimeMs == 0 {
		cfg.UpdateTimeMs = 500
	}
	if cfg.MaxConcurrentDownloads == 0 {
		cfg.MaxConcurrentDownloads = 2
	}
	if cfg.MaxFailError == 0 {
		cfg.MaxFailError = 3
	}
	return &Fafnir{
		Cfg: cfg,
	}, nil
}

func (f *Fafnir) Add(queueName, link, dwnDir, filename string) error {
	f.Cfg.Repo.Create(queueName)
	return f.Cfg.Repo.Add(queueName, link, dwnDir, filename)
}

func (f *Fafnir) StartQueueDownload(queueName string) error {
	return f.StartQueueDownloadWithCtx(context.Background(), queueName)
}

func (f *Fafnir) StartQueueDownloadWithCtx(ctx context.Context, queueName string) error {
	wg := f.Cfg.WaitGroup
	if wg == nil {
		wg = &sync.WaitGroup{}
	}
	defer wg.Wait()
	que, err := f.Cfg.Repo.Get(queueName, f.Cfg.ErrChan)
	if err != nil {
		return err
	}
	if que == nil {
		return ErrQueueNotFound
	} else if len(que.Entries) == 0 {
		return ErrEmptyQueue
	} else {
		jobsChan := make(chan Entry, f.Cfg.MaxConcurrentDownloads)

		for w := 1; w <= int(f.Cfg.MaxConcurrentDownloads); w++ {
			go f.download(ctx, wg, que, jobsChan)
		}

		for {
			if len(que.Entries) == 0 {
				// get all entries in failed entries that
				// didn't yet reach max allowed failing
				// count
				for _, fe := range que.FailedEntries {
					_, err := que.DeQueueFail()
					if err != nil {
						f.Cfg.ErrChan <- err
						continue
					}
					if fe.ExtraData.FailCount < int(f.Cfg.MaxFailError) {
						que.EnQueue(fe)
					} else {
						que.EnQueueFail(fe)
					}
				}
				if len(que.Entries) > 0 {
					continue
				}
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
			wg.Add(1)
			jobsChan <- j
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
			err2 := f.Cfg.Repo.Update(e)
			if err2 != nil {
				f.Cfg.ErrChan <- err2
			}
			f.Cfg.ErrChan <- err
			continue
		}
		req, err := grab.NewRequest(path.Join(e.DwnDir, e.Filename), e.Url)
		if err != nil {
			e.ExtraData.Err = err
			err2 := f.Cfg.Repo.Update(e)
			if err2 != nil {
				f.Cfg.ErrChan <- err2
			}
			f.Cfg.ErrChan <- err
			continue
		}
		resp := client.Do(req)

		t := time.NewTicker(time.Duration(f.Cfg.UpdateTimeMs) * time.Millisecond)
		defer t.Stop()
		func(e Entry) {
			for {
				select {
				case <-t.C:
					f.updateEntryHelper(resp, &e)
					err = f.Cfg.Repo.Update(e)
					if err != nil {
						f.Cfg.ErrChan <- err
					}
				case <-resp.Done:
					f.updateEntryHelper(resp, &e)
					// download is complete
					dlerr := resp.Err()
					if dlerr != nil {
						e.ExtraData.FailCount++
						e.ExtraData.Err = dlerr
						f.Cfg.ErrChan <- err
						queue.EnQueueFail(e)
					}
					err = f.Cfg.Repo.Update(e)
					if err != nil {
						f.Cfg.ErrChan <- err
					}
					wg.Done()
					return
				case <-ctx.Done():
					e.ExtraData.FailCount++
					err := resp.Cancel()
					if err != nil {
						f.Cfg.ErrChan <- err
					}
					queue.EnQueueFail(e)
					err = f.Cfg.Repo.Update(e)
					if err != nil {
						f.Cfg.ErrChan <- err
					}
					wg.Done()
					return
				}
			}
		}(e)
	}
}

func (f *Fafnir) updateEntryHelper(resp *grab.Response, e *Entry) {
	e.ExtraData.BytesPerSecond = resp.BytesPerSecond()
	e.ExtraData.BytesTransfered = resp.BytesComplete()
	e.ExtraData.CanResume = resp.CanResume
	e.ExtraData.DidResume = resp.DidResume
	e.ExtraData.Duration = resp.Duration()
	e.ExtraData.ETA = resp.ETA()
	e.ExtraData.End = resp.End
	e.ExtraData.Progress = resp.Progress()
	e.ExtraData.Size = resp.Size
	e.ExtraData.Start = resp.Start
}
