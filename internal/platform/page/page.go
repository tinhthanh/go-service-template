// Package page is how a list the client reads a page at a time is cut: the
// cursor is the offset as a string, as the client's lib/pages.ts writes it;
// a page is Size rows unless the client asks for another, never more than
// Max; and the next cursor is given while more are left. Every list's
// service uses it, so none cuts its pages its own way.
package page

import (
	"fmt"
	"strconv"

	"github.com/tinhthanh/go-service-template/internal/platform/problem"
)

const (
	// Size is a page's rows unless the client asks for another, as its lib/pages.ts does.
	Size = 50
	// Max is the most a page may hold: more is refused, never cut short silently.
	Max = 200
)

// Offset reads a cursor; one that isn't a number, or is past what a query's OFFSET takes (int32), starts at the top.
func Offset(cursor string) int {
	n, err := strconv.ParseInt(cursor, 10, 32)
	if err != nil || n < 0 {
		return 0
	}

	return int(n)
}

// Limit is the rows a page asked for holds: Size for none, refused on "limit" past Max.
func Limit(asked int) (int, error) {
	switch {
	case asked > Max:
		return 0, problem.Invalid("limit", fmt.Sprintf("A page holds at most %d.", Max))
	case asked > 0:
		return asked, nil
	default:
		return Size, nil
	}
}

// Next is the cursor of the page after one that started at offset and held n of total; "" on the last.
func Next(offset, n, total int) string {
	if end := offset + n; end < total {
		return strconv.Itoa(end)
	}

	return ""
}
