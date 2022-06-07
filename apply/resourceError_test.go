package apply

import (
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/xerrors"
	"testing"
)

func TestImplementsInterfaces(t *testing.T) {
	sut := NewResourceError(assert.AnError, "could not apply the thing", "Deployment", "v1", "my-deployment")
	t.Run("should implement error", func(t *testing.T) {
		require.Error(t, sut)
		require.Implements(t, (*error)(nil), sut)
	})
	t.Run("should implement Wrap", func(t *testing.T) {
		// then
		require.Implements(t, (*xerrors.Wrapper)(nil), sut)
	})
}

func TestResourceError_Error(t *testing.T) {
	t.Run("should return a full error message", func(t *testing.T) {
		// when
		sut := NewResourceError(assert.AnError, "could not apply the thing", "Deployment", "v1", "my-deployment")

		// then
		require.Error(t, sut)
		assert.Equal(t, "could not apply the thing (resource Deployment/v1/my-deployment): "+
			"assert.AnError general error for testing", sut.Error())
	})
}

func TestResourceError_Unwrap(t *testing.T) {
	sut := NewResourceError(assert.AnError, "a", "b", "c", "d")

	actual := sut.Unwrap()

	assert.Equal(t, assert.AnError, actual)
}
