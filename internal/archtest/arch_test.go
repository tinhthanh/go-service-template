// Package archtest holds the architecture's rules as a test, so a change
// that breaks one fails `make test` with what to do instead
// (docs/decisions/0002-a-package-per-feature.md):
//
//   - internal/platform knows no feature: it imports nothing of this module
//     but itself and the schema (migrations); contracttest, a test helper,
//     may read internal/api;
//   - internal/api, generated, imports only internal/platform/problem;
//   - a feature (any other internal/<name>) imports internal/api and
//     internal/platform, never another feature nor internal/server; its
//     own external tests (package <name>_test) may drive the real server;
//   - cmd/ starts the server and reads platform, nothing else.
package archtest

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"os/exec"
	"slices"
	"strings"
	"testing"
)

const _module = "github.com/tinhthanh/go-service-template"

type pkg struct {
	ImportPath   string   `json:"ImportPath"`
	Imports      []string `json:"Imports"`
	TestImports  []string `json:"TestImports"`
	XTestImports []string `json:"XTestImports"`
}

// packages is every package of the module, with what it and its tests import.
func packages(t *testing.T) []pkg {
	t.Helper()

	out, err := exec.CommandContext(t.Context(), "go", "list", "-json", _module+"/...").Output()
	if err != nil {
		t.Fatalf("go list: %v", err)
	}

	var all []pkg

	dec := json.NewDecoder(bytes.NewReader(out))

	for {
		var p pkg

		err := dec.Decode(&p)
		if errors.Is(err, io.EOF) {
			break
		}

		if err != nil {
			t.Fatalf("go list: %v", err)
		}

		all = append(all, p)
	}

	return all
}

// area is where a package sits: "platform", "api", "server", "cmd", "tools", "archtest", or "feature".
func area(path string) string {
	rel := strings.TrimPrefix(path, _module+"/")

	switch first, _, _ := strings.Cut(rel, "/"); first {
	case "cmd", "tools":
		return first
	case "internal":
		name, _, _ := strings.Cut(strings.TrimPrefix(rel, "internal/"), "/")
		if slices.Contains([]string{"platform", "api", "server", "archtest"}, name) {
			return name
		}

		return "feature"
	default:
		return "root"
	}
}

// feature is internal/<name> of a feature's package.
func feature(path string) string {
	name, _, _ := strings.Cut(strings.TrimPrefix(path, _module+"/internal/"), "/")

	return name
}

// refusal says why an import breaks the rules and what to do instead, or "".
// xtest is an import of the package's external tests (package x_test).
func refusal(from, to string, xtest bool) string {
	switch area(from) {
	case "platform":
		return fromPlatform(from, to)
	case "api":
		if to != _module+"/internal/platform/problem" {
			return "internal/api is generated from the contract and imports only internal/platform/problem: change api/openapi.json or tools/stubgen, then make generate"
		}
	case "feature":
		return fromFeature(from, to, xtest)
	case "cmd":
		if area(to) != "server" && area(to) != "platform" {
			return "cmd/ starts internal/server and reads internal/platform; anything else belongs in the server's wiring"
		}
	}

	return ""
}

func fromPlatform(from, to string) string {
	if area(to) == "platform" || to == _module+"/migrations" || (to == _module+"/internal/api" && strings.HasSuffix(from, "/contracttest")) {
		return ""
	}

	return "internal/platform knows no feature, nor the server: move what it needs into internal/platform, or pass it in as an argument"
}

func fromFeature(from, to string, xtest bool) string {
	switch area(to) {
	case "feature":
		if feature(to) != feature(from) {
			return "a feature never imports another: declare the small interface it needs in its own package, and let internal/server pass the other in"
		}
	case "server":
		if !xtest {
			return "a feature is wired by internal/server, never the other way round (its _test package may drive the server)"
		}
	case "cmd", "tools":
		return "a feature is wired by internal/server, never the other way round"
	}

	return ""
}

func TestArchitecture_EveryImportKeepsTheRules(t *testing.T) {
	t.Parallel()

	for _, p := range packages(t) {
		inner := len(p.Imports) + len(p.TestImports)

		for i, imp := range slices.Concat(p.Imports, p.TestImports, p.XTestImports) {
			if !strings.HasPrefix(imp, _module+"/") || imp == p.ImportPath {
				continue
			}

			if why := refusal(p.ImportPath, imp, i >= inner); why != "" {
				t.Errorf("%s imports %s: %s", strings.TrimPrefix(p.ImportPath, _module+"/"), strings.TrimPrefix(imp, _module+"/"), why)
			}
		}
	}
}

func TestArchitecture_TheRulesRefuseWhatTheySay(t *testing.T) {
	t.Parallel()

	m := _module + "/"

	for _, tc := range []struct {
		from, to string
		xtest    bool
		refused  bool
	}{
		{m + "internal/projects", m + "internal/tasks", false, true},
		{m + "internal/projects/store", m + "internal/projects", false, false},
		{m + "internal/projects", m + "internal/api", false, false},
		{m + "internal/projects", m + "internal/platform/problem", false, false},
		{m + "internal/projects", m + "internal/server", false, true},
		{m + "internal/platform/httpx", m + "internal/projects", false, true},
		{m + "internal/platform/httpx", m + "internal/platform/problem", false, false},
		{m + "internal/platform/contracttest", m + "internal/api", false, false},
		{m + "internal/platform/db", m + "migrations", false, false},
		{m + "internal/platform/db", m + "internal/session", false, true},
		{m + "internal/platform/httpx", m + "internal/api", false, true},
		{m + "internal/api", m + "internal/platform/problem", false, false},
		{m + "internal/api", m + "internal/platform/httpx", false, true},
		{m + "cmd/server", m + "internal/projects", false, true},
		{m + "cmd/server", m + "internal/server", false, false},
		{m + "internal/session", m + "internal/server", true, false},
		{m + "internal/session/sessiontest", m + "internal/server", false, true},
	} {
		if refused := refusal(tc.from, tc.to, tc.xtest) != ""; refused != tc.refused {
			t.Errorf("%s → %s: refused %v, want %v", tc.from, tc.to, refused, tc.refused)
		}
	}
}
