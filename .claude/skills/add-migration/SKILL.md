---
name: add-migration
description: Change the database's schema with a golang-migrate pair in migrations/, regenerate the stores' queries, and keep both directions working. Use when a feature needs a table, a column, an index or a constraint, or when asked for a migration.
---

# Add a migration

The schema is `migrations/NNNN_name.up.sql` and `.down.sql`, embedded (`migrations.FS`), applied at
start (`MIGRATE_ON_START`) and in every store test (`dbtest`), and read by sqlc to type every query.

1. **The next number**: `ls migrations` and take the next four-digit one; a name that says what it
   makes (`0003_tasks`). Never edit a migration already merged: a database that ran it won't run it
   again. Change the schema with a new one.
2. **Up**: the change. Constraints the rules rely on go here too, so the database refuses what the
   service would (a `CHECK` on a status, a unique index for a name), and every foreign key says
   what a delete does (`ON DELETE CASCADE` for a workspace's records).
3. **Down**: undoes exactly that, in the reverse order.
4. `make generate`: sqlc reads the new schema; a query it breaks fails now, by file and line.
5. **Prove it on Postgres**: a store test (`integration`) that relies on the new constraint, and
   `make verify`, which applies every migration to an empty database.
