package main

import (
	"errors"
	"fmt"
	"path/filepath"
	"time"

	"github.com/jmoiron/sqlx"
	_ "modernc.org/sqlite"
)

var (
	errNoMatches   = errors.New("no conversations found")
	errManyMatches = errors.New("multiple conversations matched the input")
)

func openDB(path string) (*convoDB, error) {
	db, err := sqlx.Open(
		"sqlite",
		"file://"+filepath.Join(path, "mods.db"),
	)
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
			updated_at datetime not null default(strftime('%Y-%m-%d %H:%M:%f', 'now')),
			check(id <> ''),
			check(title <> '')
		);
	`); err != nil {
		return nil, fmt.Errorf("could not migrate db: %w", err)
	}
	return &convoDB{db: db}, nil
}

type convoDB struct {
	db *sqlx.DB
}

// Conversation in the database.
type Conversation struct {
	ID        string    `db:"id"`
	Title     string    `db:"title"`
	UpdatedAt time.Time `db:"updated_at"`
}

func (c *convoDB) Close() error {
	return c.db.Close() //nolint: wrapcheck
}

func (c *convoDB) Save(id, title string) error {
	res, err := c.db.Exec(`
		update conversations
		set title = $2, updated_at = current_timestamp
		where id = $1
	`, id, title)
	if err != nil {
		return fmt.Errorf("Save: %w", err)
	}

	rows, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("Save: %w", err)
	}

	if rows > 0 {
		return nil
	}

	if _, err := c.db.Exec(`
		insert into conversations (id, title)
		values ($1, $2)
	`, id, title); err != nil {
		return fmt.Errorf("Save: %w", err)
	}

	return nil
}

func (c *convoDB) Delete(id string) error {
	if _, err := c.db.Exec(`
		delete from conversations
		where id = $1
	`, id); err != nil {
		return fmt.Errorf("Delete: %w", err)
	}
	return nil
}

func (c *convoDB) FindHEAD() (*Conversation, error) {
	var convo Conversation
	if err := c.db.Get(&convo, "select * from conversations order by updated_at desc limit 1"); err != nil {
		return nil, fmt.Errorf("FindHead: %w", err)
	}
	return &convo, nil
}

func (c *convoDB) Find(in string) (*Conversation, error) {
	var conversations []Conversation
	q := fmt.Sprintf(`select * from conversations where id like %q or title = %q`, in+"%", in)
	if len(in) < sha1minLen {
		q = fmt.Sprintf(`select * from conversations where title = %q`, in)
	}
	if err := c.db.Select(&conversations, q); err != nil {
		return nil, fmt.Errorf("Find: %w", err)
	}
	if len(conversations) > 1 {
		return nil, errManyMatches
	}
	if len(conversations) == 1 {
		return &conversations[0], nil
	}
	return nil, errNoMatches
}

func (c *convoDB) List() ([]Conversation, error) {
	var convos []Conversation
	if err := c.db.Select(&convos, "select * from conversations order by updated_at desc"); err != nil {
		return convos, fmt.Errorf("List: %w", err)
	}
	return convos, nil
}
