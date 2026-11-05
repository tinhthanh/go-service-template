package main

import (
	"fmt"
	"regexp"
	"slices"
	"strings"
)

// The files whose change changes what the service does.
func behaviour(f string) bool {
	switch {
	case strings.HasSuffix(f, "_test.go"), generated(f), testHelper(f):
		return false
	case strings.HasPrefix(f, "internal/archtest/"):
		return false
	case strings.HasSuffix(f, ".go"):
		return strings.HasPrefix(f, "internal/") || strings.HasPrefix(f, "cmd/")
	case strings.HasSuffix(f, ".sql"):
		return strings.HasPrefix(f, "migrations/") || strings.HasPrefix(f, "internal/")
	default:
		return false
	}
}

// generated is code a tool writes: the server from the contract, the stubs, a store's queries.
func generated(f string) bool {
	return strings.HasSuffix(f, ".gen.go") || strings.HasPrefix(f, "internal/api/") || regexp.MustCompile(`^internal/[^/]+/db/`).MatchString(f)
}

// testHelper is a package only tests use: a store in memory, servertest, dbtest, contracttest.
func testHelper(f string) bool {
	return regexp.MustCompile(`^internal/(?:[^/]+/)*[a-z]+test/`).MatchString(f)
}

func isTest(f string) bool { return strings.HasSuffix(f, "_test.go") }

// guarded is a file of the checks themselves: changing it needs the guardrail-change label.
func guarded(f string) bool {
	for _, p := range []string{
		// sqlc.yaml is not here: it lists each feature's queries, and every new feature adds to it.
		`^\.golangci\.yml$`, `^Makefile$`, `^api/oapi-codegen\.yaml$`,
		`^\.github/`, `^\.githooks/`, `^\.claude/`, `^AGENTS\.md$`, `^CLAUDE\.md$`,
		`^tools/(gates|hooks|proof|stubgen)/`, `^internal/archtest/`, `^internal/platform/(contracttest|dbtest)/`,
	} {
		if regexp.MustCompile(p).MatchString(f) {
			return true
		}
	}

	return false
}

var (
	subject = regexp.MustCompile(`^(feat|fix|test|docs|ci|chore|refactor|perf|build)(\([a-z0-9-]+\))?!?: [a-z]`)
	noTest  = regexp.MustCompile(`(?m)^no-test: (.{15,})$`)
	comment = regexp.MustCompile(`(?s)<!--.*?(-->|$)`)
	skips   = regexp.MustCompile(`\bt\.(Skip|SkipNow|Skipf)\(`)
	nolint  = regexp.MustCompile(`//\s*nolint`)
	// A nolint names its linters and says why after //: what nolintlint asks, here too
	// in case .golangci.yml is loosened in the same change.
	nolintOK = regexp.MustCompile(`//nolint:[a-z0-9_,-]+ // \S.{14,}`)
)

// PR is what a gate reads of a pull request.
type PR struct {
	Title, Body string
	Labels      []string
	Bot         bool
	Changed     []string
	// Added is each file's added lines.
	Added map[string][]string
}

// evidence is what the pull request lacks to show its change works (AGENTS.md › Done means).
func evidence(pr *PR) []string {
	var problems []string

	if pr.Title != "" && !pr.Bot && !subject.MatchString(pr.Title) {
		problems = append(problems, fmt.Sprintf("the title %q is not a lowercase conventional subject (feat: …, fix: …, docs: …)", pr.Title))
	}

	changedBehaviour := slices.DeleteFunc(slices.Clone(pr.Changed), func(f string) bool { return !behaviour(f) })
	tests := slices.ContainsFunc(pr.Changed, isTest)
	why := noTest.MatchString(pr.Body)

	switch {
	case len(changedBehaviour) > 0 && !tests && !why:
		problems = append(problems, "behaviour changed with no test changed (or a \"no-test: <why, 15 characters or more>\" line in the body):\n    "+strings.Join(changedBehaviour, "\n    "))
	case why && tests:
		problems = append(problems, "the body says no-test, but tests changed: take the line out")
	}

	if strings.HasPrefix(pr.Title, "feat") && !pr.Bot {
		for _, name := range []string{"Assumptions", "Not done"} {
			if section(pr.Body, name) == "" {
				problems = append(problems, fmt.Sprintf("a feat: pull request says under \"## %s\" what it %s (\"None.\" or \"Nothing.\" when there is nothing)", name, map[string]string{"Assumptions": "decided that the task left open", "Not done": "left out"}[name]))
			}
		}
	}

	quiet := regexp.MustCompile(`^(test|ci|chore|refactor|docs|build)(\(|:)`).MatchString(pr.Title)
	if len(changedBehaviour) > 0 && !quiet && !slices.Contains(pr.Changed, "CHANGELOG.md") {
		problems = append(problems, "a change a client would notice needs a line under ## Unreleased in CHANGELOG.md (or a title that says it isn't: test:, ci:, chore:, refactor:, docs:)")
	}

	return problems
}

// section is what the body says under "## name", its HTML comments taken out; "" when absent or empty.
func section(body, name string) string {
	m := regexp.MustCompile(`(?ms)^## ` + regexp.QuoteMeta(name) + `\s*$(.*?)(?:^## |\z)`).FindStringSubmatch(body)
	if m == nil {
		return ""
	}

	text := m[1]
	for before := ""; before != text; {
		before = text
		text = comment.ReplaceAllString(text, "")
	}

	return strings.TrimSpace(text)
}

// guardrails is how the pull request weakens a check.
func guardrails(pr *PR) []string {
	var problems []string

	touched := slices.DeleteFunc(slices.Clone(pr.Changed), func(f string) bool { return !guarded(f) })
	botBump := pr.Bot && !slices.ContainsFunc(touched, func(f string) bool { return !strings.HasPrefix(f, ".github/workflows/") })

	if len(touched) > 0 && !slices.Contains(pr.Labels, "guardrail-change") && !botBump {
		problems = append(problems, "the checks themselves changed without the \"guardrail-change\" label (say why in the body):\n    "+strings.Join(touched, "\n    "))
	}

	return append(problems, addedLines(pr)...)
}

// addedLines is each added line of Go that skips a test or silences a linter without a reason.
func addedLines(pr *PR) []string {
	var problems []string

	files := make([]string, 0, len(pr.Added))
	for f := range pr.Added {
		if strings.HasSuffix(f, ".go") && !guarded(f) && !generated(f) {
			files = append(files, f)
		}
	}

	slices.Sort(files)

	for _, f := range files {
		for _, line := range pr.Added[f] {
			if skips.MatchString(line) {
				problems = append(problems, f+": adds a t.Skip; a test that doesn't run proves nothing")
			}

			if nolint.MatchString(line) && !nolintOK.MatchString(line) {
				problems = append(problems, f+": adds a //nolint without its linter and a reason (//nolint:name // why, 15 characters or more)")
			}
		}
	}

	return problems
}
