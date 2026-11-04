package tasks_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tinhthanh/go-service-template/internal/platform/auth"
	"github.com/tinhthanh/go-service-template/internal/platform/problem"
	"github.com/tinhthanh/go-service-template/internal/tasks"
	"github.com/tinhthanh/go-service-template/internal/tasks/taskstest"
)

// Late is counted from the server's day in UTC, whatever zone its clock is read in: a task due
// yesterday in UTC is late, one due today isn't.
func TestService_LateIsCountedFromTheDayInUTC(t *testing.T) {
	t.Parallel()

	hanoi := time.FixedZone("UTC+7", 7*60*60)
	honolulu := time.FixedZone("UTC-10", -10*60*60)

	for _, c := range []struct {
		name string
		now  time.Time
		late int
	}{
		{"early on the 4th, east of UTC, is still the 3rd", time.Date(2026, 11, 4, 1, 0, 0, 0, hanoi), 0},
		{"the 4th in UTC", time.Date(2026, 11, 4, 0, 0, 0, 0, time.UTC), 1},
		{"late on the 3rd, west of UTC, is already the 4th", time.Date(2026, 11, 3, 20, 0, 0, 0, honolulu), 1},
	} {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()

			store := taskstest.New(func() time.Time { return c.now }, func(context.Context, string) (string, string, error) {
				return "", "", problem.ErrNotFound
			}, func(string, string) (tasks.Attachment, error) { return tasks.Attachment{}, problem.ErrNotFound })
			store.Add(&tasks.Task{ID: "t-1", Workspace: "w-acme", Title: "Due the 3rd", Due: on("2026-11-03")})

			ctx := auth.With(t.Context(), &auth.Principal{ID: "u-1", Role: "member", Workspaces: []string{"w-acme"}})
			page, err := tasks.New(store, func() time.Time { return c.now }).List(ctx, &tasks.Ask{})
			require.NoError(t, err)
			assert.Equal(t, c.late, page.Late)
		})
	}
}
