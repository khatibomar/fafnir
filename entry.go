package fafnir

import "time"

type EntryRepository interface {
	Update(Entry) error
	Add(string, string, string, string, string) error
	Complete(Entry) error
}

type Entry struct {
	ID          int
	Filename    string
	DwnDir      string
	Url         string
	Description string
	ExtraData   EntryExtraData
}

type EntryExtraData struct {
	FailCount       int
	Size            int64
	BytesTransfered int64
	BytesPerSecond  float64
	ETA             time.Time
	Err             error
	Duration        time.Duration
	CanResume       bool
	DidResume       bool
	Start           time.Time
	End             time.Time
	Progress        float64
}
