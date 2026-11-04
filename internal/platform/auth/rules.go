package auth

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/getkin/kin-openapi/openapi3"

	"github.com/tinhthanh/go-service-template/internal/platform/problem"
)

// What an operation's x-permission may say besides an action.
const (
	Public   = "public"
	SignedIn = "signed-in"
)

// ErrRules is a contract whose x-permission or x-roles can't be read.
var ErrRules = errors.New("auth: the contract's permissions")

// Rules are the contract's own: each operation's x-permission (public,
// signed-in, or an action) and x-roles (each action's roles). The client's
// lib/permissions.ts is held to the same table by its check:contract, so a
// role is refused here exactly where the client hides the control.
type Rules struct {
	need  map[string]string
	roles map[string][]string
}

// RulesOf reads them from the contract. Every operation must say what it
// needs, and every action it names must be in x-roles: a contract that
// doesn't fails the start.
func RulesOf(spec *openapi3.T) (*Rules, error) {
	r := &Rules{need: map[string]string{}, roles: map[string][]string{}}

	if raw, ok := spec.Extensions["x-roles"].(map[string]any); ok {
		for action, list := range raw {
			r.roles[action] = asStrings(list)
		}
	}

	for path, item := range spec.Paths.Map() {
		for method, op := range item.Operations() {
			need, _ := op.Extensions["x-permission"].(string)

			switch {
			case need == "":
				return nil, fmt.Errorf("%w: %s %s (%s) has no x-permission", ErrRules, method, path, op.OperationID)
			case need != Public && need != SignedIn && r.roles[need] == nil:
				return nil, fmt.Errorf("%w: %s's x-permission %q is not an action of x-roles", ErrRules, op.OperationID, need)
			}

			r.need[strings.ToLower(op.OperationID)] = need
		}
	}

	return r, nil
}

func asStrings(v any) []string {
	list, _ := v.([]any)
	out := make([]string, 0, len(list))

	for _, item := range list {
		if s, ok := item.(string); ok {
			out = append(out, s)
		}
	}

	return out
}

// Need is what the operation (by its operationId, in any case) asks of a caller.
func (r *Rules) Need(operation string) string { return r.need[strings.ToLower(operation)] }

// Can says whether the caller may take the action; nobody may take none,
// and an action x-roles doesn't name is nobody's.
func (r *Rules) Can(p *Principal, action string) bool {
	return p != nil && slices.Contains(r.roles[action], p.Role)
}

// Check is the operation's x-permission for the request's caller: nothing for
// public, problem.ErrUnauthorized for nobody, problem.ErrForbidden for a role
// x-roles leaves out of the action.
func (r *Rules) Check(ctx context.Context, operation string) error {
	need := r.Need(operation)
	if need == Public {
		return nil
	}

	p, err := Required(ctx)
	if err != nil {
		return err
	}

	if need != SignedIn && !r.Can(p, need) {
		return problem.ErrForbidden
	}

	return nil
}
