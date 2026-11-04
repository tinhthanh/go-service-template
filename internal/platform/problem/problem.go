// Package problem is how a refusal reaches the client: a feature returns
// one of these errors, and Write turns it into the contract's Error
// ({error, code, field}) with its status, once, for every operation.
// The raw error is logged, never sent: the words are for people.
package problem

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
)

// The refusals the client's contract names by code.
var (
	ErrUnauthorized = errors.New("not signed in")
	ErrForbidden    = errors.New("not allowed")
	ErrNotFound     = errors.New("not found")
	ErrConflict     = errors.New("conflict")
	ErrInvalid      = errors.New("invalid")
	ErrNotBuilt     = errors.New("not built yet")
)

// FieldError is ErrInvalid on one field, with the words the form puts beside it.
// Status is 422 for a value the rules refuse (Invalid), 400 for a request
// the contract refuses (Malformed): the client puts either on the field.
type FieldError struct {
	Field   string
	Message string
	Status  int
}

func (e *FieldError) Error() string { return fmt.Sprintf("invalid %s: %s", e.Field, e.Message) }

// Unwrap makes errors.Is(err, ErrInvalid) true.
func (e *FieldError) Unwrap() error { return ErrInvalid }

// Invalid refuses a value the rules don't allow: an empty name, a name taken (422).
func Invalid(field, message string) error {
	return &FieldError{Field: field, Message: message, Status: http.StatusUnprocessableEntity}
}

// Malformed refuses a request the contract doesn't allow: a field missing, of the wrong type (400).
func Malformed(field, message string) error {
	return &FieldError{Field: field, Message: message, Status: http.StatusBadRequest}
}

// Body is the contract's Error.
type Body struct {
	Error string `json:"error"`
	Code  string `json:"code,omitempty"`
	Field string `json:"field,omitempty"`
}

// Of is a refusal's status and body; anything not named above is a 500.
func Of(err error) (int, Body) {
	if field, ok := errors.AsType[*FieldError](err); ok {
		return field.Status, Body{Error: field.Message, Code: "invalid", Field: field.Field}
	}

	for _, k := range kinds() {
		if errors.Is(err, k.err) {
			return k.status, Body{Error: k.words, Code: k.code}
		}
	}

	return http.StatusInternalServerError, Body{Error: "Something went wrong on the server.", Code: "internal"}
}

type kind struct {
	err    error
	status int
	code   string
	words  string
}

func kinds() []kind {
	return []kind{
		{ErrInvalid, http.StatusBadRequest, "invalid", "Check what was entered."},
		{ErrUnauthorized, http.StatusUnauthorized, "unauthorized", "Sign in to go on."},
		{ErrForbidden, http.StatusForbidden, "forbidden", "You don't have permission to do this."},
		{ErrNotFound, http.StatusNotFound, "not_found", "It isn't there any more."},
		{ErrConflict, http.StatusConflict, "conflict", "Someone changed this first. Load it again."},
		{ErrNotBuilt, http.StatusNotImplemented, "unavailable", "This isn't built on the server yet."},
	}
}

// Logger is what Write logs a 500 to (a *slog.Logger).
type Logger interface {
	ErrorContext(ctx context.Context, msg string, args ...any)
}

// Write answers a refusal; a 500's error is logged with the request it failed.
func Write(w http.ResponseWriter, r *http.Request, log Logger, err error) {
	status, body := Of(err)
	if status >= http.StatusInternalServerError && status != http.StatusNotImplemented {
		log.ErrorContext(r.Context(), "request failed", "method", r.Method, "path", r.URL.Path, "err", err)
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)

	if encErr := json.NewEncoder(w).Encode(body); encErr != nil {
		log.ErrorContext(r.Context(), "writing a refusal", "err", encErr)
	}
}
