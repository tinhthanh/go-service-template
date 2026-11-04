package server_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tinhthanh/go-service-template/internal/api"
	"github.com/tinhthanh/go-service-template/internal/platform/auth"
)

// The client's contract, as synced: every operation says what it needs, and the roles are
// those the client's lib/permissions.ts hides by.
func TestRulesOf_TheClientsContract(t *testing.T) {
	t.Parallel()

	spec, err := api.GetSpec()
	require.NoError(t, err)

	r, err := auth.RulesOf(spec)
	require.NoError(t, err)

	assert.Equal(t, "project.delete", r.Need("deleteProject"))
	assert.Equal(t, auth.Public, r.Need("signIn"))
	assert.Equal(t, auth.SignedIn, r.Need("projects"))
	assert.False(t, r.Can(&auth.Principal{Role: "member"}, "project.delete"), "only an owner deletes")
	assert.True(t, r.Can(&auth.Principal{Role: "member"}, "project.create"))
	assert.False(t, r.Can(&auth.Principal{Role: "viewer"}, "project.create"), "a viewer only reads")
}
