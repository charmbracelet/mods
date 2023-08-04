package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/jmoiron/sqlx"
	_ "github.com/mattn/go-sqlite3"
)

func dbForConfig(cfg Config) (*convoDB, error) {
	if err := os.MkdirAll(cfg.CachePath, 0o700); err != nil {
		return nil, fmt.Errorf("could not create db: %w", err)
	}
	db, err := sqlx.Open("sqlite3", "file://"+filepath.Join(cfg.CachePath, "db.sqlite"))
	if err != nil {
		return nil, fmt.Errorf("could not create db: %w", err)
	}
	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("could not ping db: %w", err)
	}
	if _, err := db.Exec(`
		create table if not exists conversations(
			id string not null primary key,
			title string not null,
			updated_at datetime not null default current_timestamp
		);
	`); err != nil {
		return nil, fmt.Errorf("could not migrate db: %w", err)
	}
	return &convoDB{db}, nil
}

type convoDB struct {
	db *sqlx.DB
}

type dbConvo struct {
	ID        string    `db:"id"`
	Title     string    `db:"title"`
	UpdatedAt time.Time `db:"updated_at"`
}

func (c *convoDB) Save(id, title string) error {
	if _, err := c.db.Exec(`
		update conversations
		set title = $2, updated_at = current_timestamp
		where id = $1
	`, id, title); err != nil {
		return fmt.Errorf("could not save conversation: %w", err)
	}
	if _, err := c.db.Exec(`
		insert or ignore into conversations (id, title)
		values ($1, $2)
	`, id, title); err != nil {
		return fmt.Errorf("could not save conversation: %w", err)
	}

	return nil
}

func (c *convoDB) Delete(id string) error {
	if _, err := c.db.Exec(`
		delete from conversations
		where id = $1
		limit 1
	`, id); err != nil {
		return fmt.Errorf("could not delete conversation: %w", err)
	}
	return nil
}

func (c *convoDB) FindHEAD() (string, error) {
	var results string
	if err := c.db.Get(&results, "select id from conversations order by updated_at desc limit 1"); err != nil {
		return "", fmt.Errorf("could not find last conversation: %w", err)
	}
	return results, nil
}

func (c *convoDB) Find(in string) (string, error) {
	var ids []string
	q := fmt.Sprintf(`select id from conversations where id like %q or title = %q`, in+"%", in)
	if len(in) < 4 {
		q = fmt.Sprintf(`select id from conversations where title = %q`, in)
	}
	if err := c.db.Select(&ids, q); err != nil {
		return "", err
	}
	if len(ids) > 1 {
		return "", fmt.Errorf("multiple conversations matched %q: %s", in, strings.Join(ids, ", "))
	}
	if len(ids) == 1 {
		return ids[0], nil
	}
	return "", ErrNoMatches
}

var ErrNoMatches = errors.New("no conversations matched the given input")

func (c *convoDB) List() ([]dbConvo, error) {
	var convos []dbConvo
	if err := c.db.Select(&convos, "select * from conversations order by updated_at desc"); err != nil {
		return convos, fmt.Errorf("could not list conversations: %w", err)
	}
	return convos, nil
}
