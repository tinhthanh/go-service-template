// Command hooks is what Claude Code runs around an agent's work
// (.claude/settings.json), written in Go because Go is all the repository
// needs. A finding goes back to the agent on stderr with exit 2, while the
// change is fresh:
//
//	.claude/bin/hooks after-edit   after an Edit or Write: format the file, vet and lint its package
//	.claude/bin/hooks before-bash  before a shell command: note the time
//	.claude/bin/hooks after-bash   after it: the same checks on every file it changed
//	.claude/bin/hooks stop         before stopping with changes: make check
//
// A file written from the shell (a heredoc, sed -i, a script) is no Edit, so
// without before-bash and after-bash it would reach make check unformatted,
// and an edit to generated code or to the contract wouldn't be refused at
// all: trial 3 wrote all of its Go that way.
//
// It is built first, not run with go run, because go run turns the exit
// code 2 that hands a finding back into a 1, which Claude Code ignores.
package main

import (
	"encoding/json"
	"fmt"
	"io"
	"maps"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"time"
)

const (
	_tailLines = 40
	// _stamp is touched when a shell command starts; after-bash checks the files changed since.
	_stamp = ".claude/bin/bash-stamp"
)

// _makers are the commands that rewrite generated code and the contract as they should
// (make generate, make contract-sync, sqlc, the stubs' generator).
var _makers = regexp.MustCompile(`\bmake\b|go generate|go tool (sqlc|oapi-codegen)|go run \./tools/stubgen`)

type event struct {
	ToolInput struct {
		FilePath string `json:"file_path"`
		Command  string `json:"command"`
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
	case "before-bash":
		markBash()
	case "after-bash":
		finding = afterBash(ev.ToolInput.Command)
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

	if why := refused(rel); why != "" {
		return why
	}

	switch {
	case strings.HasSuffix(rel, ".sql"):
		return checkSQL()
	case strings.HasSuffix(rel, ".go"):
		if why := format(rel); why != "" {
			return why
		}

		return checkPackage("./" + filepath.ToSlash(filepath.Dir(rel)))
	}

	return ""
}

// markBash notes that a shell command is about to run.
func markBash() {
	_ = os.MkdirAll(filepath.Dir(_stamp), 0o750)
	_ = os.WriteFile(_stamp, nil, 0o600)
	now := time.Now()
	_ = os.Chtimes(_stamp, now, now)
}

// afterBash checks each file the command changed as afterEdit would, each
// package once. Generated code and the contract are refused only when the
// command isn't one that makes them.
func afterBash(command string) string {
	changed := changedSince(_stamp)
	if len(changed) == 0 {
		return ""
	}

	maker := _makers.MatchString(command)

	var findings []string

	pkgs, sql := map[string]bool{}, false

	for _, rel := range changed {
		if why := refused(rel); why != "" {
			if !maker {
				findings = append(findings, why)
			}

			continue
		}

		switch {
		case strings.HasSuffix(rel, ".sql"):
			sql = true
		case strings.HasSuffix(rel, ".go"):
			findings = append(findings, format(rel))
			pkgs["./"+filepath.ToSlash(filepath.Dir(rel))] = true
		}
	}

	if sql && !maker {
		findings = append(findings, checkSQL())
	}

	for _, pkg := range slices.Sorted(maps.Keys(pkgs)) {
		findings = append(findings, checkPackage(pkg))
	}

	return strings.Join(slices.DeleteFunc(findings, func(f string) bool { return f == "" }), "\n\n")
}

// changedSince is the files git sees changed or added, written after the stamp.
func changedSince(stamp string) []string {
	since, err := os.Stat(stamp)
	if err != nil {
		return nil
	}

	out, ok := run("git", "ls-files", "--modified", "--others", "--exclude-standard")
	if !ok {
		return nil
	}

	var changed []string

	for rel := range strings.FieldsSeq(out) {
		if info, err := os.Stat(rel); err == nil && !info.ModTime().Before(since.ModTime()) {
			changed = append(changed, rel)
		}
	}

	return changed
}

// refused says why a file isn't the agent's to write by hand, or nothing.
func refused(rel string) string {
	switch {
	case rel == filepath.Join("api", "openapi.json"):
		return "api/openapi.json is the client's contract: change it in the client's repository, then make contract-sync FROM=<it> (AGENTS.md › The contract)."
	case strings.HasSuffix(rel, ".go") && generated(rel):
		return rel + " is generated: change what it is made from (api/openapi.json, a queries.sql, a migration, tools/stubgen) and run make generate."
	}

	return ""
}

// checkSQL has sqlc type every query against the migrations, so a wrong column shows now.
func checkSQL() string {
	if out, ok := run("go", "tool", "sqlc", "generate"); !ok {
		return "sqlc refuses the queries against the migrations; fix it now:\n" + tail(out)
	}

	if out, ok := run("go", "vet", "./..."); !ok {
		return "the generated queries no longer fit the code that calls them; fix it now:\n" + tail(out)
	}

	return ""
}

// format formats one Go file in place.
func format(rel string) string {
	if out, ok := run("go", "tool", "golangci-lint", "fmt", rel); !ok {
		return rel + " couldn't be formatted:\n" + out
	}

	return ""
}

// checkPackage vets and lints one package.
func checkPackage(pkg string) string {
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
	f, err := os.Open(file) //nolint:gosec // a file of this repository the agent just changed
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
