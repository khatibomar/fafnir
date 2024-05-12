package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"path"
	"sync"
	"time"

	"github.com/khatibomar/fafnir"
	_ "github.com/ncruces/go-sqlite3"
)

const (
	createTableEntry string = `CREATE TABLE IF NOT EXISTS "entry" (
		"queue_id" INTEGER,
        "id"    INTEGER PRIMARY KEY AUTOINCREMENT,
		"filename" VARCHAR(150) NOT NULL,
		"dwn_dir" VARCHAR(150) NOT NULL,
		"description" VARCHAR(150) NOT NULL,
		"url" VARCHAR(250) NOT NULL UNIQUE,
		"fail_count"      INTEGER DEFAULT 0,
		"size" INTEGER DEFAULT 0,
		"bytes_transfered" INTEGER DEFAULT 0,
		"bytes_per_second" REAL DEFAULT 0,
        "eta"    DATETIME,
		"err"    VARCHAR(150) DEFAULT "",
		"duration"       INTEGER DEFAULT 0,
		"can_resume"	INTEGER DEFAULT 0,
		"did_resume"    INTEGER DEFAULT 0,
        "start_time"    DATETIME,
        "end_time"    DATETIME,
        "progress" REAL DEFAULT 0,
		FOREIGN KEY("queue_id") REFERENCES queue(id)
);`
	createTableQueue string = `CREATE TABLE IF NOT EXISTS "queue" (
		"id"   INTEGER PRIMARY KEY AUTOINCREMENT,
        "name"    VARCHAR(25) NOT NULL UNIQUE
);`
)

type dbRepo struct {
	db *sql.DB
	sync.RWMutex
}

func NewSQLite3Repo(dbfile, directory string) (*dbRepo, error) {
	db, err := sql.Open("sqlite3", path.Join(directory, fmt.Sprintf("%s.db", dbfile)))
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	if err := db.Ping(); err != nil {
		return nil, err
	}

	if _, err := db.Exec(createTableEntry); err != nil {
		return nil, err
	}
	if _, err := db.Exec(createTableQueue); err != nil {
		return nil, err
	}

	return &dbRepo{
		db: db,
	}, nil
}

func (r *dbRepo) Update(e fafnir.Entry) error {
	r.Lock()
	defer r.Unlock()
	updStmt, err := r.db.Prepare(
		"UPDATE entry SET description=?,filename=?, dwn_dir=?, url=?, fail_count=?,size=?,bytes_transfered=?,bytes_per_second=?,eta=?,err=?,duration=?,can_resume=?,did_resume=?,start_time=?,end_time=?,progress=? WHERE id=?")
	if err != nil {
		return err
	}
	defer updStmt.Close()

	// Exec UPDATE statement
	res, err := updStmt.Exec(e.Description, e.Filename, e.DwnDir, e.Url, e.ExtraData.FailCount, e.ExtraData.Size, e.ExtraData.BytesTransfered, e.ExtraData.BytesPerSecond, e.ExtraData.ETA, e.ExtraData.Err.Error(), e.ExtraData.Duration, e.ExtraData.CanResume, e.ExtraData.DidResume, e.ExtraData.Start, e.ExtraData.End, e.ExtraData.Progress, e.ID)
	if err != nil {
		return err
	}

	// UPDATE results
	ra, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if ra != 1 {
		return fmt.Errorf("excpected exactly 1 row to be affected got %d", ra)
	}
	return nil
}

func (r *dbRepo) Add(queueName, link, dwnDir, filename, description string) error {
	r.Lock()
	defer r.Unlock()

	ctx := context.Background()
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	var qid int
	err = tx.QueryRowContext(ctx, "SELECT id FROM queue WHERE name=?", queueName).Scan(&qid)
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, "INSERT INTO entry(queue_id , url , dwn_dir, filename, description) VALUES (?,?,?,?,?)", qid, link, dwnDir, filename, description)
	if err != nil {
		return err
	}
	if err = tx.Commit(); err != nil {
		return err
	}

	return nil
}

func (r *dbRepo) Create(queueName string) error {
	r.Lock()
	defer r.Unlock()

	// Prepare INSERT statement
	insStmt, err := r.db.Prepare("INSERT INTO queue(name) VALUES (?)")
	if err != nil {
		return err
	}
	defer insStmt.Close()

	// Exec INSERT statement
	_, err = insStmt.Exec(queueName)
	if err != nil {
		return err
	}
	return nil
}

func (r *dbRepo) Fetch() ([]*fafnir.Queue, error) {
	return nil, nil
}

func (r *dbRepo) Delete(queueName string) error {
	r.Lock()
	defer r.Unlock()

	// Prepare INSERT statement
	insStmt, err := r.db.Prepare("DELETE FROM queue WHERE name=?")
	if err != nil {
		return err
	}
	defer insStmt.Close()

	// Exec INSERT statement
	_, err = insStmt.Exec(queueName)
	if err != nil {
		return err
	}

	return nil
}

func (r *dbRepo) Get(queueName string, errChan chan error) (*fafnir.Queue, error) {
	r.Lock()
	defer r.Unlock()

	// Prepare INSERT statement
	insStmt, err := r.db.Prepare("select * from entry join queue on queue.id = entry.queue_id where queue.name = ?")
	if err != nil {
		return nil, err
	}
	defer insStmt.Close()

	// Exec INSERT statement
	var e fafnir.Entry
	q := fafnir.NewQueue(queueName)
	rows, err := insStmt.QueryContext(context.Background(), queueName)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var (
		QID             sql.NullInt32
		ID              int
		Filename        string
		DwnDir          string
		Url             string
		Description     string
		FailCount       int
		Size            int64
		BytesTransfered int64
		BytesPerSecond  float64
		ETA             sql.NullTime
		Err             string
		Duration        time.Duration
		CanResume       bool
		DidResume       bool
		Start           sql.NullTime
		End             sql.NullTime
		Progress        float64
		trash1          int
		trash2          string
	)

	for rows.Next() {
		err = rows.Scan(&QID, &ID, &Filename, &DwnDir, &Description, &Url, &FailCount, &Size, &BytesTransfered, &BytesPerSecond, &ETA, &Err, &Duration, &CanResume, &DidResume, &Start, &End, &Progress, &trash1, &trash2)
		if err != nil {
			errChan <- err
		}
		e.ID = ID
		e.Filename = Filename
		e.DwnDir = DwnDir
		e.Url = Url
		e.Description = Description
		e.ExtraData.FailCount = FailCount
		e.ExtraData.Size = Size
		e.ExtraData.BytesTransfered = BytesTransfered
		e.ExtraData.ETA = ETA.Time
		e.ExtraData.Err = errors.New(Err)
		e.ExtraData.Duration = Duration
		e.ExtraData.CanResume = CanResume
		e.ExtraData.DidResume = DidResume
		e.ExtraData.Start = Start.Time
		e.ExtraData.End = End.Time
		e.ExtraData.Progress = Progress
		q.EnQueue(e)
	}

	return q, nil
}

func (r *dbRepo) Complete(e fafnir.Entry) error {
	r.Lock()
	defer r.Unlock()

	// Prepare INSERT statement
	insStmt, err := r.db.Prepare("UPDATE entry SET queue_id=NULL where id=?")
	if err != nil {
		return err
	}
	defer insStmt.Close()

	// Exec INSERT statement
	_, err = insStmt.Exec(e.ID)
	if err != nil {
		return err
	}
	return nil
}
