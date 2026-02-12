package sqlite

import (
	"context"
	"database/sql"
	"log"
	"time"
)

const slowQueryThreshold = 100 * time.Millisecond

// dbHandle is the interface satisfied by both *sql.DB and *queryLogger.
// All Store methods use this instead of *sql.DB directly.
type dbHandle interface {
	Exec(query string, args ...any) (sql.Result, error)
	Query(query string, args ...any) (*sql.Rows, error)
	QueryRow(query string, args ...any) *sql.Row
	Begin() (*sql.Tx, error)
	BeginTx(ctx context.Context, opts *sql.TxOptions) (*sql.Tx, error)
	Close() error
}

// queryLogger wraps a *sql.DB and logs queries that exceed the slow query threshold.
type queryLogger struct {
	inner *sql.DB
}

func (q *queryLogger) Exec(query string, args ...any) (sql.Result, error) {
	start := time.Now()
	result, err := q.inner.Exec(query, args...)
	if d := time.Since(start); d >= slowQueryThreshold {
		log.Printf("SLOW QUERY (%s): %s", d.Round(time.Millisecond), truncateQuery(query))
	}
	return result, err
}

func (q *queryLogger) Query(query string, args ...any) (*sql.Rows, error) {
	start := time.Now()
	rows, err := q.inner.Query(query, args...)
	if d := time.Since(start); d >= slowQueryThreshold {
		log.Printf("SLOW QUERY (%s): %s", d.Round(time.Millisecond), truncateQuery(query))
	}
	return rows, err
}

func (q *queryLogger) QueryRow(query string, args ...any) *sql.Row {
	start := time.Now()
	row := q.inner.QueryRow(query, args...)
	d := time.Since(start)
	if d >= slowQueryThreshold {
		log.Printf("SLOW QUERY (%s): %s", d.Round(time.Millisecond), truncateQuery(query))
	}
	return row
}

func (q *queryLogger) Begin() (*sql.Tx, error) {
	return q.inner.Begin()
}

func (q *queryLogger) BeginTx(ctx context.Context, opts *sql.TxOptions) (*sql.Tx, error) {
	return q.inner.BeginTx(ctx, opts)
}

func (q *queryLogger) Close() error {
	return q.inner.Close()
}

func truncateQuery(s string) string {
	if len(s) > 200 {
		return s[:200] + "..."
	}
	return s
}
