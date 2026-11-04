// Package searchtest is the search store in memory, for tests that call the
// server without a database. It ranks as queries.sql does over the records
// Rows answers (servertest gives it the projects' and tasks' stores in memory,
// which this package may not import); the store's own tests prove the SQL.
package searchtest

import (
	"cmp"
	"context"
	"regexp"
	"slices"
	"strings"

	"github.com/tinhthanh/go-service-template/internal/search"
)

// Row is a record a search may find: its name, the rest it matches in, and its workspace.
type Row struct {
	search.Hit
	Text, Workspace string
}

// Store -.
type Store struct {
	rows func() []Row
}

var _ search.Store = (*Store)(nil)

// New is a store over the records rows answers, projects before tasks.
func New(rows func() []Row) *Store { return &Store{rows: rows} }

var words = regexp.MustCompile(`[\s_/-]+`)

// score is queries.sql's CASE.
func score(r *Row, q string) int {
	name, q := strings.ToLower(r.Name), strings.ToLower(q)

	switch {
	case name == q:
		return 100
	case strings.HasPrefix(name, q):
		return 80
	case strings.Contains(" "+words.ReplaceAllString(name, " "), " "+q):
		return 60
	case strings.Contains(name, q):
		return 40
	case strings.Contains(strings.ToLower(r.Text), q):
		return 20
	default:
		return 0
	}
}

// Find -.
func (s *Store) Find(_ context.Context, q string, workspaces []string, per int) ([]search.Hit, error) {
	out := []search.Hit{}

	for _, kind := range []string{"project", "task"} {
		type found struct {
			hit   search.Hit
			score int
		}

		var hits []found

		for _, r := range s.rows() {
			if r.Kind == kind && slices.Contains(workspaces, r.Workspace) {
				if n := score(&r, q); n > 0 {
					hits = append(hits, found{r.Hit, n})
				}
			}
		}

		slices.SortFunc(hits, func(a, b found) int {
			return cmp.Or(b.score-a.score, cmp.Compare(strings.ToLower(a.hit.Name), strings.ToLower(b.hit.Name)), cmp.Compare(a.hit.ID, b.hit.ID))
		})

		for _, h := range hits[:min(per, len(hits))] {
			out = append(out, h.hit)
		}
	}

	return out, nil
}
