package auth

import (
	"context"
	"slices"

	"github.com/tinhthanh/go-service-template/internal/platform/problem"
)

// actions is each action and the roles that may take it: the same table as
// the client's lib/permissions.ts, which hides what isn't someone's to do.
// Here is where it is decided. A new action goes in both.
func actions() map[string][]string {
	return map[string][]string{
		"project.create":   {"owner", "member"},
		"project.edit":     {"owner", "member"},
		"project.delete":   {"owner"},
		"task.create":      {"owner", "member"},
		"task.edit":        {"owner", "member"},
		"task.delete":      {"owner"},
		"file.upload":      {"owner", "member"},
		"workspace.create": {"owner", "member"},
	}
}

// Can says whether the caller may take the action; nobody may take none,
// and an action the table doesn't name is nobody's.
func Can(p *Principal, action string) bool {
	return p != nil && slices.Contains(actions()[action], p.Role)
}

// Allowed is the caller when they may take the action: problem.ErrUnauthorized
// for nobody, problem.ErrForbidden for a role that may not.
func Allowed(ctx context.Context, action string) (*Principal, error) {
	p, err := Required(ctx)
	if err != nil {
		return nil, err
	}

	if !Can(p, action) {
		return nil, problem.ErrForbidden
	}

	return p, nil
}
