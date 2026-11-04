# go-service-template

A Go backend template for AI coding agents to extend, paired with
[app-shell-template](https://github.com/tinhthanh/app-shell-template): the client owns the API
(`api/openapi.json`), and this service is generated from it and answers it.

- **Contract first.** `make contract-sync FROM=../app-shell-template` copies the client's contract
  in; `make generate` turns it into a typed net/http server (oapi-codegen strict). Every operation
  is routed; one not built yet answers 501 `unavailable`. Every request is validated against the
  contract, and every answer a test reads is checked against it.
- **A package per feature**, its boundaries a test (`internal/archtest`).
- **The standard library first**: net/http, log/slog.
- **Checks an agent can't skip**: a hook after each edit (format, vet, lint the package), `make
  check` before it stops, git hooks, and CI that is `make verify`.

```sh
make setup      # once: the git hooks
make check      # generated code current, lint, every test
make run        # the server on :8080
```

`AGENTS.md` is how to work here; `docs/decisions/` is why it is this way.
