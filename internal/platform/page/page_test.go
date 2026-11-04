package page_test

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tinhthanh/go-service-template/internal/platform/page"
	"github.com/tinhthanh/go-service-template/internal/platform/problem"
)

func TestOffset_ACursorThatIsntOneStartsAtTheTop(t *testing.T) {
	t.Parallel()

	for cursor, want := range map[string]int{"": 0, "50": 50, "abc": 0, "-1": 0, "99999999999": 0} {
		assert.Equal(t, want, page.Offset(cursor), cursor)
	}
}

func TestLimit_IsTheSizeUnlessAskedAndNeverPastTheMost(t *testing.T) {
	t.Parallel()

	n, err := page.Limit(0)
	require.NoError(t, err)
	assert.Equal(t, page.Size, n)

	n, err = page.Limit(page.Max)
	require.NoError(t, err)
	assert.Equal(t, page.Max, n)

	_, err = page.Limit(page.Max + 1)
	status, body := problem.Of(err)
	assert.Equal(t, http.StatusUnprocessableEntity, status)
	assert.Equal(t, "limit", body.Field)
}

func TestNext_IsGivenWhileMoreAreLeft(t *testing.T) {
	t.Parallel()

	assert.Equal(t, "2", page.Next(0, 2, 3))
	assert.Empty(t, page.Next(2, 1, 3), "the last page")
	assert.Empty(t, page.Next(9, 0, 3), "a page past the end")
}
