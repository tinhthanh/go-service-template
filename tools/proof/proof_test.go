package main

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func lsTree(files ...string) string {
	lines := make([]string, 0, len(files))
	for _, f := range files {
		lines = append(lines, "100644 blob "+strings.Repeat("a", 40)+"\t"+f)
	}

	return strings.Join(lines, "\n")
}

func record(t *testing.T, change func(r *Record)) []byte {
	t.Helper()

	r := Record{Commit: "abc1234", Tree: treeOf(lsTree("internal/a.go")), Steps: []Step{{Name: "make verify", OK: true}}, Review: "ready"}
	change(&r)

	raw, err := json.Marshal(r)
	require.NoError(t, err)

	return raw
}

func TestProof_CommittingTheRecordAfterTheRunLeavesTheTreeTheRunChecked(t *testing.T) {
	t.Parallel()

	assert.Equal(t, treeOf(lsTree("internal/a.go")), treeOf(lsTree("internal/a.go", "evidence/verify.json", "evidence/review.md")))
	assert.NotEqual(t, treeOf(lsTree("internal/a.go")), treeOf(lsTree("internal/a.go", "internal/b.go")))
}

func TestProof_AGreenReviewedRunOfThisTreeIsProofAndNothingElseIs(t *testing.T) {
	t.Parallel()

	tree := treeOf(lsTree("internal/a.go"))

	assert.Empty(t, problems(record(t, func(*Record) {}), tree))
	assert.Contains(t, problems(nil, tree)[0], "no evidence/verify.json")
	assert.Contains(t, problems([]byte("{"), tree)[0], "isn't JSON")
	assert.Contains(t, problems(record(t, func(r *Record) { r.Tree = "another" }), tree)[0], "another tree")
	assert.Contains(t, problems(record(t, func(r *Record) { r.Steps = []Step{{Name: "make verify"}} }), tree)[0], "red: make verify")
	assert.Contains(t, problems(record(t, func(r *Record) { r.Steps = nil }), tree)[0], "names no step")
	assert.Contains(t, problems(record(t, func(r *Record) { r.Review = "not ready" }), tree)[0], `"not ready", not "ready"`)
}

func TestProof_TheReviewersVerdictIsReadFromItsOwnLine(t *testing.T) {
	t.Parallel()

	assert.Equal(t, "ready", verdictOf("VERDICT: ready\n\nNo findings."))
	assert.Equal(t, "not ready", verdictOf("Some preamble\nVERDICT: not ready: no test\n- internal/x.go:3"))
	assert.Equal(t, "none", verdictOf("Looks ready to me."))
}

func TestProof_TheReviewerIsGivenOnlyToolsThatReadAndNoSettingsThatWouldLetMoreThrough(t *testing.T) {
	t.Parallel()

	args, err := reviewArgs(promptOf(nil), "---\nname: reviewer\ndescription: Reads a diff.\ntools: Read, Bash\n---\n\nYou review.\n")
	require.NoError(t, err)

	after := func(flag string) string { return args[indexOf(args, flag)+1] }
	assert.Empty(t, after("--setting-sources"))
	assert.Equal(t, "dontAsk", after("--permission-mode"))
	assert.Contains(t, args, "--strict-mcp-config")
	assert.Equal(t, "Bash(*--output*)", after("--disallowedTools"))
	assert.JSONEq(t, `{"reviewer": {"description": "Reads a diff.", "prompt": "You review.", "tools": ["Read", "Grep", "Glob", "Bash"]}}`, after("--agents"))

	for _, tool := range reviewTools() {
		assert.Regexp(t, `^(Read|Grep|Glob|Bash\((git (diff|log|show)|gh pr (view|diff)):\*\))$`, tool)
	}

	_, err = reviewArgs("", "no frontmatter")
	assert.ErrorContains(t, err, "frontmatter")
}

func TestProof_TheReviewReadsThePullRequestsTextAndWithoutOneJudgesTheCodeAlone(t *testing.T) {
	t.Parallel()

	assert.Contains(t, promptOf(&PR{Title: "ci: a title", Body: "The body."}), "ci: a title\n\nThe body.")
	assert.Contains(t, promptOf(nil), "no pull request text yet")
}

func indexOf(args []string, flag string) int {
	for i, a := range args {
		if a == flag {
			return i
		}
	}

	return -1
}
