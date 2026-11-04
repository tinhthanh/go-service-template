package httpx

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
)

type nullsKey struct{}

// Nulls notes which top-level fields of a PATCH's JSON body are null. The
// contract's "null takes it off" (anyOf with null) is generated as a pointer,
// so a handler sees nil for null and for left out alike; SentNull tells them
// apart. The body is put back as it was for whatever reads it next; one that
// isn't a JSON object notes nothing (the contract refuses it after).
func Nulls() Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodPatch || r.Body == nil {
				next.ServeHTTP(w, r)

				return
			}

			body, err := io.ReadAll(r.Body)
			_ = r.Body.Close()
			r.Body = io.NopCloser(bytes.NewReader(body))

			var fields map[string]json.RawMessage
			if err != nil || json.Unmarshal(body, &fields) != nil {
				next.ServeHTTP(w, r)

				return
			}

			nulls := map[string]bool{}

			for k, v := range fields {
				if string(bytes.TrimSpace(v)) == "null" {
					nulls[k] = true
				}
			}

			next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), nullsKey{}, nulls)))
		})
	}
}

// SentNull says whether the request's body set the field to null (Nulls).
func SentNull(ctx context.Context, field string) bool {
	nulls, _ := ctx.Value(nullsKey{}).(map[string]bool)

	return nulls[field]
}
