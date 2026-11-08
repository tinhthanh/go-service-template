package httpx

import (
	"bytes"
	"encoding/json/jsontext"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"mime"
	"net/http"
	"slices"
	"strings"

	"github.com/tinhthanh/go-service-template/internal/platform/problem"
)

// WellFormed refuses a JSON body that encoding/json would take but change or
// guess at (ADR 0016): text that isn't UTF-8, which it keeps with U+FFFD in
// place of what was sent; a lone surrogate (`"\ud800"`), the same; a name
// sent twice in one object, where it keeps the last; and anything after the
// value. jsontext (encoding/json/v2's own reader) holds the body to RFC 8259
// as it is, and says where it broke: the refusal is 400 invalid, on the
// top-level field it broke in when there is one. The body is put back as it
// was for whatever reads it next; a body that isn't JSON (by its
// Content-Type) or is empty goes on for the contract to judge, and one cut
// short by MaxBody is a 413.
func WellFormed(log *slog.Logger) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Body == nil || r.Body == http.NoBody || !isJSON(r.Header.Get("Content-Type")) {
				next.ServeHTTP(w, r)

				return
			}

			body, err := io.ReadAll(r.Body)
			_ = r.Body.Close()

			if err != nil {
				problem.Write(w, r, log, fmt.Errorf("httpx: reading the body: %w", err))

				return
			}

			if err := wellFormed(body); err != nil {
				problem.Write(w, r, log, err)

				return
			}

			r.Body = io.NopCloser(bytes.NewReader(body))
			next.ServeHTTP(w, r)
		})
	}
}

// wellFormed is nil for one JSON value as RFC 8259 has it, and nothing after;
// an empty body is left to the contract.
func wellFormed(body []byte) error {
	if len(bytes.TrimSpace(body)) == 0 {
		return nil
	}

	d := jsontext.NewDecoder(bytes.NewReader(body))

	err := d.SkipValue()
	if err == nil {
		if _, err = d.ReadToken(); errors.Is(err, io.EOF) {
			return nil
		}
	}

	if syntax, ok := errors.AsType[*jsontext.SyntacticError](err); ok {
		if at := slices.Collect(syntax.JSONPointer.Tokens()); len(at) > 0 {
			return problem.Malformed(at[0], "This value isn't accepted here.")
		}
	}

	return fmt.Errorf("%w: %w", problem.ErrInvalid, err)
}

// isJSON is application/json, or a type that says it is JSON (+json).
func isJSON(contentType string) bool {
	media, _, err := mime.ParseMediaType(contentType)

	return err == nil && (media == "application/json" || strings.HasSuffix(media, "+json"))
}
