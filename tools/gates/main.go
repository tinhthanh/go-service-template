// Command gates reads a pull request as CI does and says what it lacks:
//
//	go run ./tools/gates evidence     a test for a change, the changelog, the title, a feat's Assumptions and Not done
//	go run ./tools/gates guardrails   no t.Skip or bare //nolint added; the checks changed only with the guardrail-change label
//
// CI gives it the PR in env: PR_TITLE, PR_BODY, PR_LABELS (comma-separated; with
// PR_NUMBER the labels are read from the PR as it is now, with gh),
// PR_ACTOR and BASE_SHA. Locally the same env, with the diff against
// origin/main (make gates; the open-pr skill).
package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "gates: evidence or guardrails")
		os.Exit(2)
	}

	pr, err := read()
	if err != nil {
		fmt.Fprintln(os.Stderr, "gates:", err)
		os.Exit(2)
	}

	var problems []string

	switch os.Args[1] {
	case "evidence":
		problems = evidence(pr)
	case "guardrails":
		problems = guardrails(pr)
	default:
		fmt.Fprintln(os.Stderr, "gates: evidence or guardrails")
		os.Exit(2)
	}

	if len(problems) > 0 {
		fmt.Fprintf(os.Stderr, "gates %s: the pull request lacks what AGENTS.md › Done means asks:\n\n  %s\n", os.Args[1], strings.Join(problems, "\n  "))
		os.Exit(1)
	}

	fmt.Printf("gates %s: %d files changed, nothing lacking\n", os.Args[1], len(pr.Changed))
}

// read is the pull request: its text from env, its files and added lines from git.
func read() (*PR, error) {
	base := os.Getenv("BASE_SHA")
	if base == "" {
		base = "origin/main"
	}

	changed, err := git("diff", "--name-only", base+"...HEAD")
	if err != nil {
		return nil, err
	}

	diff, err := git("diff", "--unified=0", base+"...HEAD")
	if err != nil {
		return nil, err
	}

	pr := &PR{
		Title: os.Getenv("PR_TITLE"), Body: os.Getenv("PR_BODY"), Bot: os.Getenv("PR_ACTOR") == "dependabot[bot]",
		Changed: strings.Fields(changed), Added: added(diff),
	}

	for l := range strings.SplitSeq(labels(), ",") {
		if l = strings.TrimSpace(l); l != "" {
			pr.Labels = append(pr.Labels, l)
		}
	}

	return pr, nil
}

// labels is the PR's labels as they are now, read from GitHub when CI names
// the PR (PR_NUMBER): opening a PR with a label starts two runs, and the one
// the label's event started can be the one cancelled, leaving a run whose
// event saw none. PR_LABELS when gh can't say (locally, before the PR exists).
func labels() string {
	if n := os.Getenv("PR_NUMBER"); n != "" {
		out, err := exec.CommandContext(context.Background(), "gh", "pr", "view", n, "--json", "labels", "-q", `[.labels[].name] | join(",")`).Output() //nolint:gosec // gh with this program's own arguments and the PR's number
		if err == nil {
			return strings.TrimSpace(string(out))
		}
	}

	return os.Getenv("PR_LABELS")
}

// added is each file's added lines, from a unified diff.
func added(diff string) map[string][]string {
	out := map[string][]string{}
	file := ""

	for line := range strings.SplitSeq(diff, "\n") {
		switch {
		case strings.HasPrefix(line, "+++ "):
			file = strings.TrimPrefix(strings.TrimPrefix(line, "+++ "), "b/")
		case strings.HasPrefix(line, "+") && file != "":
			out[file] = append(out[file], line[1:])
		}
	}

	return out
}

func git(args ...string) (string, error) {
	out, err := exec.Command("git", args...).Output() //nolint:noctx,gosec // git with this program's own arguments, run to its end
	if err != nil {
		return "", fmt.Errorf("git %s: %w", strings.Join(args, " "), err)
	}

	return string(out), nil
}
