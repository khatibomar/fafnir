## Fafnir
<img style="width:250px;height:150px;" src=".github/assets/logo.gif"/><br>
Fafnir is a library that acts as a download manager with advanced features for your App

## Usage example

```go
package main

import (
	"sync"

	"github.com/khatibomar/fafnir"
	"github.com/khatibomar/fafnir/repository"
)

func main() {
	errChan := make(chan error, 10)
	go func() {
		for err := range errChan {
			// handle error
		}
	}()

	repo := repository.NewInMemory()

	var wg sync.WaitGroup

	cfg := &fafnir.Config{
		ErrChan:                errChan,
		MaxConcurrentDownloads: 2,
		MaxFailError:           3,
		UpdateTimeMs:           1000,
		Repo:                   repo,
		WaitGroup:              &wg,
	}
	faf, err := fafnir.New(cfg)
	if err != nil {
		// handle error
	}

	queueName := "q1"
	downloadDir := "./downloads"
	faf.Add(
		queueName,
		"https://static.wikia.nocookie.net/maid-dragon/images/f/ff/Fafnir_profile.jpg",
		downloadDir,
		"fafnir.jpg",
		"Fafnir (ファフニール Fafunīru)",
	)

	faf.Add(
		queueName,
		"https://static.wikia.nocookie.net/maid-dragon/images/5/57/Kanna_Anime.png",
		downloadDir,
		"kanna.png",
		"Kanna Kamui (カンナカムイ) ",
	)

	if err = faf.StartQueueDownload(queueName); err != nil {
		// handle error
	}
}
```

## Showcase

https://github.com/khatibomar/fafnir/assets/35725554/4efa7cd2-2cf8-42fb-8de7-1b2e8f2487b6
