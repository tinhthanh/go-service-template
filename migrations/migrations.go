// Package migrations is the database's schema, as golang-migrate pairs
// (NNNN_name.up.sql and .down.sql), embedded so the server and the tests
// apply the same files, and read by sqlc to type every query.
package migrations

import "embed"

// FS is every migration.
//
//go:embed *.sql
var FS embed.FS
