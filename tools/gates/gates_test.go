package main

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

const _feat = "feat: tasks answer the client"

func body(sections ...string) string {
	return strings.Join(sections, "\n\n")
}

func TestEvidence_ABehaviourChangeNeedsATest(t *testing.T) {
	t.Parallel()

	pr := &PR{Title: "fix: a refused save names its field", Changed: []string{"internal/projects/service.go", "CHANGELOG.md"}}
	assert.Len(t, evidence(pr), 1)
	assert.Contains(t, evidence(pr)[0], "internal/projects/service.go")

	pr.Changed = append(pr.Changed, "internal/projects/handler_test.go")
	assert.Empty(t, evidence(pr))
}

func TestEvidence_ANoTestLineNeedsAReasonAndNoTest(t *testing.T) {
	t.Parallel()

	pr := &PR{Title: "fix: x", Changed: []string{"internal/platform/httpx/middleware.go", "CHANGELOG.md"}, Body: "no-test: short"}
	assert.Len(t, evidence(pr), 1, "a reason under 15 characters is none")

	pr.Body = "no-test: the logger's format only, nothing a test reads"
	assert.Empty(t, evidence(pr))

	pr.Changed = append(pr.Changed, "internal/platform/httpx/middleware_test.go")
	assert.Contains(t, strings.Join(evidence(pr), " "), "take the line out")
}

func TestEvidence_GeneratedCodeTestHelpersAndDocsAreNoBehaviour(t *testing.T) {
	t.Parallel()

	pr := &PR{Title: "docs: the skills", Changed: []string{
		"internal/api/api.gen.go", "internal/projects/db/queries.sql.gen.go", "internal/projects/projectstest/store.go",
		"internal/server/servertest/servertest.go", "internal/archtest/arch_test.go", "AGENTS.md", "docs/decisions/0012-x.md",
	}}

	assert.Empty(t, evidence(pr))
}

func TestEvidence_AQueryOrAMigrationIsBehaviour(t *testing.T) {
	t.Parallel()

	for _, f := range []string{"migrations/0003_tasks.up.sql", "internal/tasks/queries.sql", "cmd/server/main.go"} {
		assert.NotEmpty(t, evidence(&PR{Title: "chore: x", Changed: []string{f}}), f)
	}
}

func TestEvidence_ATitleIsALowercaseConventionalSubject(t *testing.T) {
	t.Parallel()

	assert.NotEmpty(t, evidence(&PR{Title: "Add tasks"}))
	assert.NotEmpty(t, evidence(&PR{Title: "feat: Add tasks"}))
	assert.Empty(t, evidence(&PR{Title: "Bump the all group", Bot: true}), "dependabot can't be told")
	assert.Empty(t, evidence(&PR{Title: "chore(deps): bump x"}))
}

func TestEvidence_AFeatSaysWhatItDecidedAndLeftOut(t *testing.T) {
	t.Parallel()

	pr := &PR{Title: _feat, Body: body("## Assumptions\n\n<!-- What the task left open -->", "## Not done\n\nNothing.")}
	assert.Equal(t, []string{`a feat: pull request says under "## Assumptions" what it decided that the task left open ("None." or "Nothing." when there is nothing)`}, evidence(pr), "the template's comment is no answer")

	pr.Body = body("## Assumptions\n\nNone.", "## Not done\n\nNothing.")
	assert.Empty(t, evidence(pr))
}

func TestEvidence_AChangeAClientNoticesHasAChangelogLine(t *testing.T) {
	t.Parallel()

	pr := &PR{Title: "fix: x", Changed: []string{"internal/projects/service.go", "internal/projects/service_test.go"}}
	assert.Contains(t, strings.Join(evidence(pr), " "), "CHANGELOG.md")

	pr.Title = "refactor: x"
	assert.Empty(t, evidence(pr), "a refactor says it isn't one")
}

func TestGuardrails_TheChecksChangeOnlyWithTheLabel(t *testing.T) {
	t.Parallel()

	for _, f := range []string{".golangci.yml", "Makefile", ".github/workflows/ci.yml", ".claude/settings.json", "AGENTS.md", "internal/archtest/arch_test.go", "tools/gates/gates.go", "internal/platform/contracttest/contracttest.go"} {
		assert.NotEmpty(t, guardrails(&PR{Changed: []string{f}}), f)
		assert.Empty(t, guardrails(&PR{Changed: []string{f}, Labels: []string{"guardrail-change"}}), f)
	}

	assert.Empty(t, guardrails(&PR{Changed: []string{".github/workflows/ci.yml"}, Bot: true}), "dependabot bumps an action")
	assert.Empty(t, guardrails(&PR{Changed: []string{"sqlc.yaml"}}), "a new feature's queries are no change to the checks")
	assert.NotEmpty(t, guardrails(&PR{Changed: []string{".golangci.yml"}, Bot: true}))
}

func TestGuardrails_ASkippedTestOrABareNolintIsRefused(t *testing.T) {
	t.Parallel()

	for line, refused := range map[string]bool{
		`	t.Skip("flaky")`:              true,
		`	t.Skipf("later %d", n)`:       true,
		`	x := 1 //nolint`:              true,
		`	x := 1 //nolint:mnd`:          true,
		`	x := 1 //nolint:mnd // short`: true,
		`	x := 1 //nolint:mnd // a page's size, never near overflow`: false,
		`	// t.Skip is what we never do`:                             false,
	} {
		pr := &PR{Added: map[string][]string{"internal/projects/service.go": {line}}}
		assert.Equal(t, refused, len(guardrails(pr)) > 0, line)
	}
}

func TestAdded_ReadsEachFilesAddedLines(t *testing.T) {
	t.Parallel()

	got := added("diff --git a/x.go b/x.go\n--- a/x.go\n+++ b/x.go\n@@ -1 +1,2 @@\n-old\n+new\n+more\n")

	assert.Equal(t, map[string][]string{"x.go": {"new", "more"}}, got)
}

func TestGates_WithNoPullRequestNamedTheLabelsAreTheOnesGiven(t *testing.T) {
	t.Setenv("PR_NUMBER", "")
	t.Setenv("PR_LABELS", "guardrail-change,dependencies")

	assert.Equal(t, "guardrail-change,dependencies", labels())
}
