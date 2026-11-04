-- name: CreateFile :one
INSERT INTO files (owner_id, name, type, size, data)
VALUES (@owner_id, @name, @type, @size, @data)
RETURNING id, name, type, size;
