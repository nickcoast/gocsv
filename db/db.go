package db

import (
	"context"
	"database/sql"
	"embed"
)

//go:embed evolutions/*.sql
var evolutionsFS embed.FS

type DB struct {
	db *sql.DB
}

func NewDB(connectionString string) (*DB, error) {
	db, err := sql.Open("postgres", connectionString)
	if err != nil {
		return nil, err
	}

	err = db.Ping()
	if err != nil {
		return nil, err
	}

	return &DB{db: db}, nil
}

func (d *DB) Close() {
	d.db.Close()
}

func (d *DB) QueryWithContext(ctx context.Context, query string, args ...interface{}) (*sql.Rows, error) {
	/* ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	   defer cancel() */

	rows, err := d.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}

	return rows, nil
}
func (d *DB) QueryRowContext(ctx context.Context, query string, args ...interface{}) *sql.Row {
	/* ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	   defer cancel() */

	return d.db.QueryRowContext(ctx, query, args...)
}

type Tx struct {
	tx *sql.Tx
	db *DB
}

func (d *DB) BeginTx(ctx context.Context) (*Tx, error) {
	/* ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	   defer cancel() */

	tx, err := d.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}

	return &Tx{tx: tx, db: d}, nil
}

func (t *Tx) Rollback() error {
	return t.tx.Rollback()
}

func (t *Tx) Commit() error {
	return t.tx.Commit()
}

func (t *Tx) QueryContext(ctx context.Context, query string, args ...interface{}) (*sql.Rows, error) {
	rows, err := t.tx.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}

	return rows, nil
}

func (t *Tx) QueryRowContext(ctx context.Context, query string, args ...interface{}) *sql.Row {
	/*  ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	    defer cancel() */

	return t.tx.QueryRowContext(ctx, query, args...)
}
func (t *Tx) ExecContext(ctx context.Context, query string, args ...interface{}) (sql.Result, error) {
	res, err := t.tx.ExecContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}

	return res, nil
}

func (t *Tx) PrepareContext(ctx context.Context, query string) (*sql.Stmt, error) {
	return t.tx.PrepareContext(ctx, query)
}
