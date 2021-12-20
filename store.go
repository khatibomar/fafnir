package fafnir

type Store interface {
	Set()
	Get()
}

type SQLliteDB struct{}

func NewSQLliteDB() *SQLliteDB {
	return &SQLliteDB{}
}

func (sq *SQLliteDB) Set(filename, path, dwnDir string) {}

func (sq *SQLliteDB) Get() {}
