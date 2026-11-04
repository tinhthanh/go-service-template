//go:build integration

package search_test

import (
	"fmt"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tinhthanh/go-service-template/internal/platform/dbtest"
	"github.com/tinhthanh/go-service-template/internal/search"
)

// db has Acme's projects and a task, and Beta's Shop, which an Acme search never finds.
func db(t *testing.T) (*pgxpool.Pool, *search.PGStore) {
	t.Helper()

	pool := dbtest.New(t)

	for _, q := range []string{
		`INSERT INTO workspaces (id, name) VALUES ('w-acme', 'Acme'), ('w-beta', 'Beta')`,
		`INSERT INTO projects (workspace_id, name, description, owner) VALUES
		   ('w-acme', 'Blog', 'A shop-ish thing', 'Max'), ('w-acme', 'Workshop', '', 'Max'),
		   ('w-acme', 'Shopfront redesign', '', 'Max'), ('w-acme', 'Shop', '', 'Olive'),
		   ('w-acme', 'pet_shop/site', '', 'Kim'), ('w-acme', '50% off', '', 'Kim'), ('w-beta', 'Shop', '', 'Beta')`,
		`INSERT INTO tasks (workspace_id, title, project_id) SELECT 'w-acme', 'Price the shop', id FROM projects WHERE name = 'Blog'`,
	} {
		_, err := pool.Exec(t.Context(), q)
		require.NoError(t, err)
	}

	return pool, search.NewPGStore(pool)
}

func found(t *testing.T, s *search.PGStore, q string) []string {
	t.Helper()

	hits, err := s.Find(t.Context(), q, []string{"w-acme"}, search.PerKind)
	require.NoError(t, err)

	out := []string{}
	for _, h := range hits {
		out = append(out, fmt.Sprintf("%s:%s (%s)", h.Kind, h.Name, h.Detail))
	}

	return out
}

func TestPGStore_RanksAsTheClientDoes(t *testing.T) {
	t.Parallel()

	_, s := db(t)

	assert.Equal(t, []string{
		"project:Shop (Olive)",             // the name itself
		"project:Shopfront redesign (Max)", // starts with it
		"project:pet_shop/site (Kim)",      // a word starts with it, words split at _ / - and spaces
		"project:Workshop (Max)",           // has it
		"project:Blog (Max)",               // the rest has it
		"task:Price the shop (Blog)",       // tasks on their own, the detail their project's name
	}, found(t, s, "SHOP"))
}

func TestPGStore_EachKindIsAtMostPerKind(t *testing.T) {
	t.Parallel()

	pool, s := db(t)

	_, err := pool.Exec(t.Context(), `INSERT INTO projects (workspace_id, name, owner) SELECT 'w-acme', 'Shop ' || n, 'x' FROM generate_series(1, 9) n`)
	require.NoError(t, err)

	hits, err := s.Find(t.Context(), "shop", []string{"w-acme"}, search.PerKind)
	require.NoError(t, err)

	projects := 0
	for _, h := range hits {
		if h.Kind == "project" {
			projects++
		}
	}

	assert.Equal(t, search.PerKind, projects)
	assert.Equal(t, "task", hits[len(hits)-1].Kind, "eight projects don't crowd the task out")
}

func TestPGStore_WhatWasTypedIsNeverAPattern(t *testing.T) {
	t.Parallel()

	_, s := db(t)

	assert.Equal(t, []string{"project:50% off (Kim)"}, found(t, s, "50%"))
	assert.Equal(t, []string{"project:50% off (Kim)"}, found(t, s, "%"), "a % is the one name that has one, not every name")
	assert.Equal(t, []string{"project:pet_shop/site (Kim)"}, found(t, s, "_"), "a _ is the one name that has one, not any letter")
	assert.Empty(t, found(t, s, ".*"))
}
