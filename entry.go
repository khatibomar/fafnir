package fafnir

import "time"

type EntryRepository interface {
	Update(Entry) error
	Add(string, Entry) error
}

type Entry struct {
	ID        int
	Filename  string
	DwnDir    string
	Url       string
	ExtraData EntryExtraData
}

type EntryExtraData struct {
	Size            uint64
	BytesTransfered uint64
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
