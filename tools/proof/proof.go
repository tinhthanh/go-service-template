package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"slices"
	"strings"
)

// The refusals, said by main.
var (
	errNoFrontmatter = errors.New("the reviewer's file has no frontmatter")
	errUsage         = errors.New("run, check, or check --ref <sha>")
	errNoCommit      = errors.New("no such commit")
	errNoProof       = errors.New("no green run of its own (ADR 0015)")
	errDirty         = errors.New("uncommitted changes: the run and its record are of a commit, so commit first")
	errRed           = errors.New("red (evidence/ says so; fix, commit, run again)")
)

// The record make proof writes, and the reviewer's answer beside it.
const (
	recordPath = "evidence/verify.json"
	reviewPath = "evidence/review.md"
)

// Step is one thing the run did, and how it went.
type Step struct {
	Name    string `json:"name"`
	OK      bool   `json:"ok"`
	Seconds int    `json:"seconds"`
}

// Record is evidence/verify.json.
type Record struct {
	Commit string `json:"commit"`
	Tree   string `json:"tree"`
	At     string `json:"at"`
	Steps  []Step `json:"steps"`
	Review string `json:"review"`
	Go     string `json:"go"`
	OS     string `json:"os"`
}

// treeOf hashes `git ls-tree -r` of a commit, leaving out evidence/: the
// files a run checked, whatever record of it was committed after.
func treeOf(lsTree string) string {
	var lines []string

	for line := range strings.SplitSeq(lsTree, "\n") {
		_, path, ok := strings.Cut(line, "\t")
		if ok && !strings.HasPrefix(path, "evidence/") {
			lines = append(lines, line)
		}
	}

	slices.Sort(lines)
	sum := sha256.Sum256([]byte(strings.Join(lines, "\n")))

	return hex.EncodeToString(sum[:])
}

var _verdict = regexp.MustCompile(`(?im)^\s*VERDICT:\s*(ready|not ready)\b`)

// verdictOf reads the first line that starts with VERDICT: ("ready" or "not ready …"), wherever the reviewer puts it.
func verdictOf(review string) string {
	m := _verdict.FindStringSubmatch(review)
	if m == nil {
		return "none"
	}

	return strings.ToLower(m[1])
}

// problems says what keeps a record from being this tree's green, reviewed run; none when it is.
func problems(raw []byte, tree string) []string {
	if raw == nil {
		return []string{"no " + recordPath + ": run make proof, then commit evidence/"}
	}

	var r Record
	if err := json.Unmarshal(raw, &r); err != nil {
		return []string{recordPath + " isn't JSON: " + err.Error()}
	}

	var out []string

	if r.Tree != tree {
		out = append(out, fmt.Sprintf("%s is a run of another tree (commit %.7s): run make proof on this one", recordPath, r.Commit))
	}

	if len(r.Steps) == 0 {
		out = append(out, recordPath+" names no step it ran")
	}

	for _, s := range r.Steps {
		if !s.OK {
			out = append(out, "a step was red: "+s.Name)
		}
	}

	if r.Review != "ready" {
		out = append(out, fmt.Sprintf("the reviewer's verdict is %q, not \"ready\" (%s says why)", r.Review, reviewPath))
	}

	return out
}

// reviewLoaded is the tools the reviewer has at all.
func reviewLoaded() []string { return []string{"Read", "Grep", "Glob", "Bash"} }

// reviewTools is the ones it may use: it reads, and asks git and gh.
func reviewTools() []string {
	return []string{"Read", "Grep", "Glob", "Bash(git diff:*)", "Bash(git log:*)", "Bash(git show:*)", "Bash(gh pr view:*)", "Bash(gh pr diff:*)"}
}

// reviewDenied: git diff, log and show write a file given --output, so it is denied, as a redirect already is.
func reviewDenied() []string { return []string{"Bash(*--output*)"} }

// PR is the pull request's text the review reads, when there is one.
type PR struct{ Title, Body string }

// promptOf is what the review is asked: make proof runs before the push, so
// often there is no pull request text yet, and the gates check it when there is.
func promptOf(pr *PR) string {
	text := "There is no pull request text yet (make proof runs before the push): judge the code and its tests, " +
		"don't hold the missing text against it, and say in a few lines what the pull request must say (its label, Assumptions, Not done)."
	if pr != nil {
		text = "The pull request's text, to check as your instructions say:\n\n" + pr.Title + "\n\n" + pr.Body
	}

	return "Review this branch as the reviewer agent does: its diff is `git diff origin/main...HEAD`.\n" + text + "\n" +
		"You are the last step of make proof, whose record (" + recordPath + ", committed with the branch after you) carries each step's result: " +
		"a Proof that points at it for this run's results, rather than pasting them, is as asked.\n" +
		"Begin your answer with one line, exactly `VERDICT: ready` or `VERDICT: not ready: <the reason, in a few words>`.\n" +
		"Ready means nothing you found must be fixed before it merges; a finding worth a follow-up is not a reason to say not ready.\n" +
		"Then the findings, most serious first, each with file:line, as your instructions say."
}

var _frontmatter = regexp.MustCompile(`(?s)^---\n(.*?)\n---\n(.*)$`)

// agentOf reads an agent's file as --agents takes it: with no settings
// loaded, the project's agents aren't either, so the reviewer is handed over whole.
func agentOf(md string) (map[string]any, error) {
	m := _frontmatter.FindStringSubmatch(md)
	if m == nil {
		return nil, errNoFrontmatter
	}

	description := ""

	for line := range strings.SplitSeq(m[1], "\n") {
		if d, ok := strings.CutPrefix(line, "description:"); ok {
			description = strings.TrimSpace(d)
		}
	}

	return map[string]any{"description": description, "prompt": strings.TrimSpace(m[2]), "tools": reviewLoaded()}, nil
}

// reviewArgs is claude's command line for the review: read-only by more than
// its word, since only the reading tools are loaded, no settings (the
// project's allowlist lets git commit and make through) and no MCP server,
// and in dontAsk mode anything not in reviewTools is denied.
func reviewArgs(prompt, reviewerMd string) ([]string, error) {
	agent, err := agentOf(reviewerMd)
	if err != nil {
		return nil, err
	}

	agents, err := json.Marshal(map[string]any{"reviewer": agent})
	if err != nil {
		return nil, fmt.Errorf("proof: the reviewer: %w", err)
	}

	args := []string{"-p", prompt, "--agents", string(agents), "--agent", "reviewer", "--setting-sources", "", "--strict-mcp-config", "--permission-mode", "dontAsk", "--tools"}
	args = append(args, reviewLoaded()...)
	args = append(append(args, "--allowedTools"), reviewTools()...)
	args = append(append(args, "--disallowedTools"), reviewDenied()...)

	return append(args, "--output-format", "text"), nil
}
