package main

import (
	"errors"
	"fmt"
	"time"

	"github.com/jmoiron/sqlx"
	"modernc.org/sqlite"
)

var (
	errNoMatches   = errors.New("no conversations found")
	errManyMatches = errors.New("multiple conversations matched the input")
)

func handleSqliteErr(err error) error {
	sqerr := &sqlite.Error{}
	if errors.As(err, &sqerr) {
		return fmt.Errorf(
			"%w: %s",
			sqerr,
			sqlite.ErrorCodeString[sqerr.Code()],
		)
	}
	return err
}

func openDB(ds string) (*convoDB, error) {
	db, err := sqlx.Open("sqlite", ds)
	if err != nil {
		return nil, fmt.Errorf(
			"could not create db: %w",
			handleSqliteErr(err),
		)
	}
	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf(
			"could not ping db: %w",
			handleSqliteErr(err),
		)
	}
	if _, err := db.Exec(`
		create table if not exists conversations(
			id string not null primary key,
			title string not null,
			updated_at datetime not null default(strftime('%Y-%m-%d %H:%M:%f', 'now')),
			check(id <> ''),
			check(title <> '')
		)
	`); err != nil {
		return nil, fmt.Errorf("could not migrate db: %w", err)
	}
	if _, err := db.Exec(`
		create index if not exists idx_conv_id
		on conversations(id)
	`); err != nil {
		return nil, fmt.Errorf("could not migrate db: %w", err)
	}
	if _, err := db.Exec(`
		create index if not exists idx_conv_title
		on conversations(title)
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
	res, err := c.db.Exec(c.db.Rebind(`
		update conversations
		set title = ?, updated_at = current_timestamp
		where id = ?
	`), title, id)
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

	if _, err := c.db.Exec(c.db.Rebind(`
		insert into conversations (id, title)
		values (?, ?)
	`), id, title); err != nil {
		return fmt.Errorf("Save: %w", err)
	}

	return nil
}

func (c *convoDB) Delete(id string) error {
	if _, err := c.db.Exec(c.db.Rebind(`
		delete from conversations
		where id = ?
	`), id); err != nil {
		return fmt.Errorf("Delete: %w", err)
	}
	return nil
}

func (c *convoDB) FindHEAD() (*Conversation, error) {
	var convo Conversation
	if err := c.db.Get(&convo, `
		select *
		from conversations
		order by updated_at desc
		limit 1
	`); err != nil {
		return nil, fmt.Errorf("FindHead: %w", err)
	}
	return &convo, nil
}

func (c *convoDB) findByExactTitle(result *[]Conversation, in string) error {
	if err := c.db.Select(result, c.db.Rebind(`
		select *
		from conversations
		where title = ?
	`), in); err != nil {
		return fmt.Errorf("findByExactTitle: %w", err)
	}
	return nil
}

func (c *convoDB) findByIDOrTitle(result *[]Conversation, in string) error {
	if err := c.db.Select(result, c.db.Rebind(`
		select *
		from conversations
		where id glob ?
		or title = ?
	`), in+"*", in); err != nil {
		return fmt.Errorf("findByIDOrTitle: %w", err)
	}
	return nil
}

func (c *convoDB) Completions(in string) ([]string, error) {
	var result []string
	if err := c.db.Select(&result, c.db.Rebind(`
		select printf(
			'%s%c%s',
			case
			when length(?) < ? then
				substr(id, 1, ?)
			else
				id
			end,
			char(9),
			title
		)
		from conversations where id glob ?
		union
		select
			printf(
				"%s%c%s",
				title,
				char(9),
				substr(id, 1, ?)
		)
		from conversations
		where title glob ?
	`), in, sha1short, sha1short, in+"*", sha1short, in+"*"); err != nil {
		return result, fmt.Errorf("Completions: %w", err)
	}
	return result, nil
}

func (c *convoDB) Find(in string) (*Conversation, error) {
	var conversations []Conversation
	var err error

	if len(in) < sha1minLen {
		err = c.findByExactTitle(&conversations, in)
	} else {
		err = c.findByIDOrTitle(&conversations, in)
	}
	if err != nil {
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
	if err := c.db.Select(&convos, `
		select *
		from conversations
		order by updated_at desc
	`); err != nil {
		return convos, fmt.Errorf("List: %w", err)
	}
	return convos, nil
}
