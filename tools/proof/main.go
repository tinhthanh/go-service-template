// Command proof is CI on a laptop, for a repository where Actions minutes
// cost (the repository variable CI_MODE=local; ADR 0015), and a record of
// it that the push and the pull request check:
//
//	go run ./tools/proof run     make verify on a clean copy of HEAD, the gates, then the reviewer agent; writes evidence/
//	go run ./tools/proof check   evidence/verify.json is a green, reviewed run of HEAD's tree (--ref <sha> for another commit)
//
// make proof and make proof-check run them. The tree is every tracked file
// but evidence/, so committing the record after the run leaves it this
// tree's. The gates read the pull request from PR_TITLE, PR_BODY and
// PR_LABELS, as make gates does; the review reads the same, or gh pr view,
// or PR.md.
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "proof: run or check")
		os.Exit(2)
	}

	var err error

	switch os.Args[1] {
	case "run":
		err = run(context.Background())
	case "check":
		ref := "HEAD"
		if len(os.Args) > 3 && os.Args[2] == "--ref" {
			ref = os.Args[3]
		}

		err = check(context.Background(), ref)
	default:
		err = errUsage
	}

	if err != nil {
		fmt.Fprintln(os.Stderr, "proof:", err)
		os.Exit(1)
	}
}

// check holds a commit to the record committed in it.
func check(ctx context.Context, ref string) error {
	listed, err := git(ctx, "ls-tree", "-r", ref)
	if err != nil {
		return fmt.Errorf("%w: %s", errNoCommit, ref)
	}

	raw, err := git(ctx, "show", ref+":"+recordPath)
	if err != nil {
		raw = ""
	}

	var record []byte
	if raw != "" {
		record = []byte(raw)
	}

	if p := problems(record, treeOf(listed)); len(p) > 0 {
		return fmt.Errorf("%s has %w:\n  %s", ref, errNoProof, strings.Join(p, "\n  "))
	}

	fmt.Println("proof:", recordPath, "is a green run of this tree, reviewed ready")

	return nil
}

// run is make verify on a clean copy of HEAD, the gates and the review, and their record.
func run(ctx context.Context) error {
	status, err := git(ctx, "status", "--porcelain")
	if err != nil {
		return err
	}

	for line := range strings.SplitSeq(status, "\n") {
		if len(line) > 3 && !strings.HasPrefix(line[3:], "evidence/") {
			return errDirty
		}
	}

	commit, err := git(ctx, "rev-parse", "HEAD")
	if err != nil {
		return err
	}

	listed, err := git(ctx, "ls-tree", "-r", "HEAD")
	if err != nil {
		return err
	}

	commit = strings.TrimSpace(commit)
	steps := []Step{verify(ctx)}

	_, _ = git(ctx, "fetch", "-q", "origin", "main")
	for _, gate := range []string{"evidence", "guardrails"} {
		steps = append(steps, step(ctx, "go run ./tools/gates "+gate, ".", "go", "run", "./tools/gates", gate))
	}

	said, verdict, took := review(ctx)
	steps = append(steps, Step{Name: "the review (claude -p --agent reviewer)", OK: verdict == "ready", Seconds: took})

	record := Record{Commit: commit, Tree: treeOf(listed), At: time.Now().UTC().Format(time.RFC3339), Steps: steps, Review: verdict, Go: runtime.Version(), OS: runtime.GOOS + "/" + runtime.GOARCH}
	if err := write(&record, said); err != nil {
		return err
	}

	return summary(steps)
}

// write is evidence/: the reviewer's answer and the record.
func write(record *Record, said string) error {
	if err := os.MkdirAll("evidence", 0o750); err != nil {
		return fmt.Errorf("evidence: %w", err)
	}

	header := fmt.Sprintf("<!-- The reviewer agent's answer on %.7s, written by make proof (tools/proof). -->\n\n", record.Commit)
	if err := os.WriteFile(reviewPath, []byte(header+strings.TrimSpace(said)+"\n"), 0o600); err != nil {
		return fmt.Errorf("evidence: %w", err)
	}

	raw, err := json.MarshalIndent(record, "", "  ")
	if err != nil {
		return fmt.Errorf("evidence: %w", err)
	}

	if err := os.WriteFile(recordPath, append(raw, '\n'), 0o600); err != nil {
		return fmt.Errorf("evidence: %w", err)
	}

	return nil
}

// summary prints each step's mark, and is errRed when one was red.
func summary(steps []Step) error {
	fmt.Println("\nproof: what it says")

	red := false

	for _, s := range steps {
		mark := "✓"
		if !s.OK {
			mark, red = "✗", true
		}

		fmt.Printf("  %s %s (%ds)\n", mark, s.Name, s.Seconds)
	}

	if red {
		return errRed
	}

	fmt.Println("proof: green. Commit evidence/ (git add evidence && git commit -m 'chore: proof'), then push")

	return nil
}

// verify is make verify on a clean copy of HEAD, as CI sees only what is committed.
func verify(ctx context.Context) Step {
	dir, err := os.MkdirTemp("", "proof-")
	if err != nil {
		return Step{Name: "make verify (no copy of HEAD: " + err.Error() + ")"}
	}

	copyOf := filepath.Join(dir, "head")
	defer func() {
		_, _ = git(ctx, "worktree", "remove", "--force", copyOf)
		_ = os.RemoveAll(dir)
	}()

	if _, err := git(ctx, "worktree", "add", "--detach", copyOf, "HEAD"); err != nil {
		return Step{Name: "make verify (no copy of HEAD: " + err.Error() + ")"}
	}

	return step(ctx, "make verify (a clean copy of HEAD)", copyOf, "make", "verify")
}

// review asks the reviewer agent, read-only, and answers what it said, its verdict and how long it took.
func review(ctx context.Context) (said, verdict string, seconds int) {
	start := time.Now()

	md, err := os.ReadFile(filepath.Join(".claude", "agents", "reviewer.md"))
	if err != nil {
		return "VERDICT: not ready: no .claude/agents/reviewer.md", "not ready", 0
	}

	args, err := reviewArgs(promptOf(prOf(ctx)), string(md))
	if err != nil {
		return "VERDICT: not ready: " + err.Error(), "not ready", 0
	}

	claude := os.Getenv("CLAUDE_BIN")
	if claude == "" {
		claude = "claude"
	}

	fmt.Println("\nproof: the review (Claude Code's reviewer agent, read-only)")

	cmd := exec.CommandContext(ctx, claude, args...) //nolint:gosec // claude with this program's own arguments
	cmd.Stderr = os.Stderr
	out, err := cmd.Output()
	took := int(time.Since(start).Seconds())

	if err != nil {
		said = fmt.Sprintf("VERDICT: not ready: the review didn't run to its end (%v); install Claude Code or set CLAUDE_BIN\n\n%s", err, out)
	} else {
		said = string(out)
	}

	fmt.Println(strings.TrimSpace(said))

	return said, verdictOf(said), took
}

// prOf is the pull request's text: PR_TITLE and PR_BODY, the PR on GitHub, or PR.md; nil when there is none yet.
func prOf(ctx context.Context) *PR {
	if t := os.Getenv("PR_TITLE"); t != "" {
		return &PR{Title: t, Body: os.Getenv("PR_BODY")}
	}

	if out, err := exec.CommandContext(ctx, "gh", "pr", "view", "--json", "title,body").Output(); err == nil {
		var pr struct {
			Title string `json:"title"`
			Body  string `json:"body"`
		}
		if json.Unmarshal(out, &pr) == nil && pr.Title != "" {
			return &PR{Title: pr.Title, Body: pr.Body}
		}
	}

	if raw, err := os.ReadFile("PR.md"); err == nil {
		title, body, _ := strings.Cut(string(raw), "\n")

		return &PR{Title: title, Body: strings.TrimSpace(body)}
	}

	return nil
}

// step runs a command where it is told, its output on the terminal, and says how it went.
func step(ctx context.Context, name, dir, command string, args ...string) Step {
	fmt.Println("\nproof:", name)

	start := time.Now()
	cmd := exec.CommandContext(ctx, command, args...) //nolint:gosec // this program's own commands
	cmd.Dir, cmd.Stdout, cmd.Stderr = dir, os.Stdout, os.Stderr
	err := cmd.Run()

	return Step{Name: name, OK: err == nil, Seconds: int(time.Since(start).Seconds())}
}

func git(ctx context.Context, args ...string) (string, error) {
	out, err := exec.CommandContext(ctx, "git", args...).Output() //nolint:gosec // git with this program's own arguments
	if err != nil {
		return "", fmt.Errorf("git %s: %w", strings.Join(args, " "), err)
	}

	return string(out), nil
}
