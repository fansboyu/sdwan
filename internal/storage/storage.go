package storage

import (
	"context"
	_ "embed"
	"errors"

	"englishlisten/sdwan/internal/storage/sqlc"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

//go:embed migrations/000001_init.up.sql
var initMigration string

type Store struct {
	pool    *pgxpool.Pool
	Queries *sqlc.Queries
}

func Open(ctx context.Context, databaseURL string) (*Store, error) {
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		return nil, err
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, err
	}
	return &Store{pool: pool, Queries: sqlc.New(pool)}, nil
}

func (s *Store) Close() {
	if s != nil && s.pool != nil {
		s.pool.Close()
	}
}

func (s *Store) Migrate(ctx context.Context) error {
	if s == nil || s.pool == nil {
		return errors.New("storage is not open")
	}
	_, err := s.pool.Exec(ctx, initMigration)
	return err
}

func (s *Store) WithTx(ctx context.Context, fn func(*sqlc.Queries) error) error {
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if err := fn(sqlc.New(tx)); err != nil {
		return err
	}
	return tx.Commit(ctx)
}
