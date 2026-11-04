package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStubs_OneStubPerOperationOfTheInterface(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "api.gen.go")
	require.NoError(t, os.WriteFile(path, []byte(`package api

type StrictServerInterface interface {
	Projects(ctx context.Context, request ProjectsRequestObject) (ProjectsResponseObject, error)
	Me(ctx context.Context, request MeRequestObject) (MeResponseObject, error)
}
`), 0o600))

	src, err := stubs(path)
	require.NoError(t, err)

	assert.Contains(t, string(src), "func (Unimplemented) Projects(context.Context, ProjectsRequestObject) (ProjectsResponseObject, error)")
	assert.Contains(t, string(src), "func (Unimplemented) Me(")
	assert.Equal(t, 2, strings.Count(string(src), "return nil, problem.ErrNotBuilt"))
	assert.True(t, strings.HasPrefix(string(src), "// Code generated"), "marked generated, so linters and people leave it")
}

func TestStubs_AFileWithoutTheInterfaceIsRefused(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "x.go")
	require.NoError(t, os.WriteFile(path, []byte("package api\n"), 0o600))

	_, err := stubs(path)
	require.ErrorIs(t, err, errNoInterface)
}
