// Package db is the service's Postgres: a pgx pool, and the migrations
// (migrations/, embedded) applied to it with golang-migrate, at start and in tests.
package db

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/pgx/v5" // the pgx5:// driver
	"github.com/golang-migrate/migrate/v4/source/iofs"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/tinhthanh/go-service-template/migrations"
)

// Connect opens a pool and checks it answers.
func Connect(ctx context.Context, url string) (*pgxpool.Pool, error) {
	pool, err := pgxpool.New(ctx, url)
	if err != nil {
		return nil, fmt.Errorf("db: %w", err)
	}

	if err := pool.Ping(ctx); err != nil {
		pool.Close()

		return nil, fmt.Errorf("db: %w", err)
	}

	return pool, nil
}

// Migrate applies every migration not yet applied; none left is no error.
func Migrate(url string) error {
	src, err := iofs.New(migrations.FS, ".")
	if err != nil {
		return fmt.Errorf("db: migrations: %w", err)
	}

	m, err := migrate.NewWithSourceInstance("iofs", src, migrateURL(url))
	if err != nil {
		return fmt.Errorf("db: migrate: %w", err)
	}

	defer func() { _, _ = m.Close() }()

	if err := m.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		return fmt.Errorf("db: migrate: %w", err)
	}

	return nil
}

// migrateURL is the URL with golang-migrate's pgx5 scheme.
func migrateURL(url string) string {
	for _, prefix := range []string{"postgres://", "postgresql://"} {
		if rest, ok := strings.CutPrefix(url, prefix); ok {
			return "pgx5://" + rest
		}
	}

	return url
}

// foreignKeyViolation is Postgres's code for a row that names one that isn't there.
const foreignKeyViolation = "23503"

// Missing says whether a write failed because a row it names was deleted
// since the service checked it (a task deleted while a comment was being
// written on it): the store answers problem.ErrNotFound for that, as the
// check would have, never a 500. constraint, when given, narrows it to that
// foreign key, so a write naming two rows says which one went.
func Missing(err error, constraint ...string) bool {
	var pg *pgconn.PgError
	if !errors.As(err, &pg) || pg.Code != foreignKeyViolation {
		return false
	}

	return len(constraint) == 0 || slices.Contains(constraint, pg.ConstraintName)
}
