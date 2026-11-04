// Package dbtest is a real Postgres for a store's tests (build tag
// integration, run by make verify): one container per test binary, the
// migrations applied once to a template database, and each test given a
// database of its own cloned from it, so tests run in parallel and none
// sees another's rows. Needs Docker.
package dbtest

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/testcontainers/testcontainers-go/modules/postgres"

	"github.com/tinhthanh/go-service-template/internal/platform/db"
)

const (
	_image    = "postgres:18-alpine"
	_template = "migrated"
)

// shared is the one container and its template, started by the first test that asks.
//
//nolint:gochecknoglobals // one container for the test binary, started once
var shared struct {
	once  sync.Once
	admin string
	err   error
	n     atomic.Int64
}

// New is a pool on a database of the test's own, with every migration applied.
func New(t *testing.T) *pgxpool.Pool {
	t.Helper()

	shared.once.Do(func() { shared.admin, shared.err = start() })

	if shared.err != nil {
		t.Fatalf("dbtest: Postgres in Docker: %v", shared.err)
	}

	name := fmt.Sprintf("t%d", shared.n.Add(1))

	admin, err := pgx.Connect(t.Context(), shared.admin)
	if err != nil {
		t.Fatalf("dbtest: %v", err)
	}

	defer func() { _ = admin.Close(context.Background()) }()

	if _, err := admin.Exec(t.Context(), "CREATE DATABASE "+name+" TEMPLATE "+_template); err != nil {
		t.Fatalf("dbtest: a database for the test: %v", err)
	}

	pool, err := db.Connect(t.Context(), withDatabase(shared.admin, name))
	if err != nil {
		t.Fatalf("dbtest: %v", err)
	}

	t.Cleanup(pool.Close)

	return pool
}

// start runs the container and migrates the template database.
func start() (string, error) {
	ctx := context.Background()

	c, err := postgres.Run(ctx, _image,
		postgres.WithDatabase(_template), postgres.WithUsername("acme"), postgres.WithPassword("acme"),
		postgres.BasicWaitStrategies())
	if err != nil {
		return "", fmt.Errorf("start: %w", err)
	}

	url, err := c.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		return "", fmt.Errorf("start: %w", err)
	}

	if err := db.Migrate(url); err != nil {
		return "", err
	}

	// A template is copied only while nobody is connected to it.
	return withDatabase(url, "postgres"), nil
}

// withDatabase is the URL naming another database on the same server.
func withDatabase(url, name string) string {
	head, query, _ := strings.Cut(url, "?")
	head = head[:strings.LastIndex(head, "/")+1] + name

	if query != "" {
		return head + "?" + query
	}

	return head
}
