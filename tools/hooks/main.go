// Command hooks is what Claude Code runs around an agent's work
// (.claude/settings.json), written in Go because Go is all the repository
// needs. A finding goes back to the agent on stderr with exit 2, while the
// change is fresh:
//
//	.claude/bin/hooks after-edit   after an Edit or Write: format the file, vet and lint its package
//	.claude/bin/hooks stop         before stopping with changes: make check
//
// It is built first, not run with go run, because go run turns the exit
// code 2 that hands a finding back into a 1, which Claude Code ignores.
package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

const _tailLines = 40

type event struct {
	ToolInput struct {
		FilePath string `json:"file_path"`
	} `json:"tool_input"`
	StopHookActive bool `json:"stop_hook_active"`
}

func main() {
	var ev event

	raw, _ := io.ReadAll(os.Stdin)
	_ = json.Unmarshal(raw, &ev)

	if len(os.Args) < 2 {
		os.Exit(0)
	}

	var finding string

	switch os.Args[1] {
	case "after-edit":
		finding = afterEdit(ev.ToolInput.FilePath)
	case "stop":
		finding = stop(ev.StopHookActive)
	}

	if finding != "" {
		fmt.Fprintln(os.Stderr, finding)
		os.Exit(2)
	}
}

// afterEdit checks the one file and its package, and says what is wrong.
func afterEdit(file string) string {
	root, err := os.Getwd()
	if err != nil || file == "" {
		return ""
	}

	rel, err := filepath.Rel(root, file)
	if err != nil || strings.HasPrefix(rel, "..") {
		return ""
	}

	switch {
	case rel == filepath.Join("api", "openapi.json"):
		return "api/openapi.json is the client's contract: change it in the client's repository, then make contract-sync FROM=<it> (AGENTS.md › The contract)."
	case strings.HasSuffix(rel, ".go") && generated(file):
		return rel + " is generated: change what it is made from (api/openapi.json, a queries.sql, a migration, tools/stubgen) and run make generate."
	case strings.HasSuffix(rel, ".sql"):
		// A query or a migration: sqlc types every query against the schema, so a wrong column shows now.
		if out, ok := run("go", "tool", "sqlc", "generate"); !ok {
			return "sqlc refuses the queries against the migrations; fix it now:\n" + tail(out)
		}

		if out, ok := run("go", "vet", "./..."); !ok {
			return "the generated queries no longer fit the code that calls them; fix it now:\n" + tail(out)
		}

		return ""
	case !strings.HasSuffix(rel, ".go"):
		return ""
	}

	if out, ok := run("go", "tool", "golangci-lint", "fmt", rel); !ok {
		return rel + " couldn't be formatted:\n" + out
	}

	pkg := "./" + filepath.ToSlash(filepath.Dir(rel))
	if out, ok := run("go", "vet", pkg); !ok {
		return pkg + " fails go vet; fix it now:\n" + tail(out)
	}

	if out, ok := run("go", "tool", "golangci-lint", "run", pkg); !ok {
		return pkg + " breaks a lint rule (.golangci.yml); fix it now:\n" + tail(out)
	}

	return ""
}

// stop runs make check when the tree has changes. A stop already blocked
// once goes through, so a check that cannot pass doesn't loop.
func stop(active bool) string {
	if active {
		return ""
	}

	if out, ok := run("git", "status", "--porcelain"); !ok || strings.TrimSpace(out) == "" {
		return ""
	}

	if out, ok := run("make", "check"); !ok {
		return "make check is red; fix it before you stop (AGENTS.md › Done means):\n\n" + tail(out)
	}

	return ""
}

// generated says whether the file says it is generated, as Go's convention writes it.
func generated(file string) bool {
	f, err := os.Open(file) //nolint:gosec // the file the agent just edited, in this repository
	if err != nil {
		return false
	}
	defer func() { _ = f.Close() }()

	head := make([]byte, 512) //nolint:mnd // enough for the first lines
	n, _ := f.Read(head)

	return strings.Contains(string(head[:n]), "Code generated") && strings.Contains(string(head[:n]), "DO NOT EDIT")
}

func run(name string, args ...string) (string, bool) {
	out, err := exec.Command(name, args...).CombinedOutput() //nolint:noctx,gosec // the commands are this program's own, never input; Claude Code times the hook out

	return string(out), err == nil
}

func tail(s string) string {
	lines := strings.Split(strings.TrimRight(s, "\n"), "\n")

	return strings.Join(lines[max(0, len(lines)-_tailLines):], "\n")
}
