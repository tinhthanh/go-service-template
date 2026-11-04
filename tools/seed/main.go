// Command seed puts the acme people and their workspace in the database, so
// a laptop can sign in: an owner, a member and a viewer of w-acme, each
// with SEED_PASSWORD (acme-password when unset). Synthetic, never real
// people; running it again changes nothing.
//
//	DATABASE_URL=… go run ./tools/seed
package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"

	"golang.org/x/crypto/bcrypt"

	"github.com/tinhthanh/go-service-template/internal/platform/db"
)

var errNoDatabase = errors.New("seed: DATABASE_URL is not set")

func main() {
	if err := run(context.Background()); err != nil {
		log.Fatal(err)
	}

	log.Printf("seed: owner@acme.test, member@acme.test and viewer@acme.test, of w-acme")
}

func run(ctx context.Context) error {
	url := os.Getenv("DATABASE_URL")
	if url == "" {
		return errNoDatabase
	}

	password := os.Getenv("SEED_PASSWORD")
	if password == "" {
		password = "acme-password"
	}

	if err := db.Migrate(url); err != nil {
		return err
	}

	pool, err := db.Connect(ctx, url)
	if err != nil {
		return err
	}
	defer pool.Close()

	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("seed: %w", err)
	}

	for _, q := range []struct {
		sql  string
		args []any
	}{
		{`INSERT INTO workspaces (id, name) VALUES ('w-acme', 'Acme') ON CONFLICT DO NOTHING`, nil},
		{`INSERT INTO users (name, email, password_hash, role) VALUES
  ('Olive Owner', 'owner@acme.test', $1, 'owner'),
  ('Max Member', 'member@acme.test', $1, 'member'),
  ('Vic Viewer', 'viewer@acme.test', $1, 'viewer')
ON CONFLICT DO NOTHING`, []any{string(hash)}},
		{`INSERT INTO memberships (user_id, workspace_id) SELECT id, 'w-acme' FROM users WHERE email LIKE '%@acme.test' ON CONFLICT DO NOTHING`, nil},
	} {
		if _, err := pool.Exec(ctx, q.sql, q.args...); err != nil {
			return fmt.Errorf("seed: %w", err)
		}
	}

	return nil
}
