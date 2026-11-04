package api_test

import (
	"context"
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tinhthanh/go-service-template/internal/api"
	"github.com/tinhthanh/go-service-template/internal/platform/problem"
)

// Every operation of the contract has a stub, and each answers that it isn't built (a 501 the
// client reads as unavailable): an operation the contract gains does until a feature builds it.
func TestUnimplemented_AnswersEveryOperationNotBuiltYet(t *testing.T) {
	t.Parallel()

	ops := reflect.TypeFor[api.StrictServerInterface]()
	stubs := reflect.ValueOf(api.Unimplemented{})

	require.Positive(t, ops.NumMethod())

	for method := range ops.Methods() {
		name := method.Name
		m := stubs.MethodByName(name)
		require.True(t, m.IsValid(), "no stub for %s", name)

		out := m.Call([]reflect.Value{reflect.ValueOf(context.Background()), reflect.New(m.Type().In(1)).Elem()})
		err, _ := reflect.TypeAssert[error](out[1])

		assert.ErrorIs(t, err, problem.ErrNotBuilt, name)
	}
}
