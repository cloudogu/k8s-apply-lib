package apply

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/rest"
)

const testFieldManagerName = "my-app-controller"

func TestNew(t *testing.T) {
	t.Run("should create a new Applier", func(t *testing.T) {
		actual, scheme, _ := New(&rest.Config{}, testFieldManagerName)

		require.NotNil(t, actual)
		assert.NotNil(t, scheme)
	})
	t.Run("should fail for empty field manager name", func(t *testing.T) {
		_, _, err := New(&rest.Config{}, "")

		require.Error(t, err)
		assert.Contains(t, err.Error(), "fieldManager must not be empty")
	})
}

func Test_Applier_implements_interface(t *testing.T) {
	sut, _, err := New(&rest.Config{}, testFieldManagerName)

	require.NoError(t, err)
	assert.Implements(t, (*applier)(nil), sut)
}

func Test_Applier_Apply(t *testing.T) {
	t.Run("should create new namespaced resource with PATCH", func(t *testing.T) {
		// given
		expectedResourceGroupKind := schema.GroupKind{Group: "", Kind: "ServiceAccount"}
		mockedRestMapping := &meta.RESTMapping{
			Resource: schema.GroupVersionResource{
				Group:    "",
				Version:  "v1",
				Resource: "the-best-resource-in-store",
			},
			GroupVersionKind: schema.GroupVersionKind{
				Group:   "",
				Version: "v1",
				Kind:    "ServiceAccount",
			},
			Scope: meta.RESTScopeNamespace,
		}
		gvrMapperMock := newMockGvrMapper(t)
		gvrMapperMock.EXPECT().RESTMapping(expectedResourceGroupKind, "v1").Return(mockedRestMapping, nil)

		parsedJsonResult := make(map[string]interface{})
		unstructuredResultMock := &unstructured.Unstructured{Object: parsedJsonResult}

		apiInterfaceMock := newMockNamespaceInterface(t)
		apiInterfaceMock.EXPECT().Namespace(mock.Anything).
			Return(apiInterfaceMock)
		apiInterfaceMock.EXPECT().Patch(mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
			Return(unstructuredResultMock, nil)

		dynClientMock := newMockDynClient(t)
		dynClientMock.EXPECT().Resource(mock.Anything).Return(apiInterfaceMock)

		sut := Applier{
			gvrMapper: gvrMapperMock,
			dynClient: dynClientMock,
		}

		testResource := []byte(`apiVersion: v1
kind: ServiceAccount
metadata:
  name: the-best-resource-in-store
  namespace: ecosystem`)

		// when
		err := sut.Apply(testResource, "mynamespace")

		// then
		require.NoError(t, err)
	})
	t.Run("should create new global resource with PATCH", func(t *testing.T) {
		// given
		expectedResourceGroupKind := schema.GroupKind{Group: "", Kind: "Namespace"}
		mockedRestMapping := &meta.RESTMapping{
			Resource: schema.GroupVersionResource{
				Group:    "",
				Version:  "v1",
				Resource: "the-best-resource-in-store",
			},
			GroupVersionKind: schema.GroupVersionKind{
				Group:   "",
				Version: "v1",
				Kind:    "Namespace",
			},
			Scope: meta.RESTScopeRoot,
		}
		gvrMapperMock := newMockGvrMapper(t)
		gvrMapperMock.EXPECT().RESTMapping(expectedResourceGroupKind, "v1").Return(mockedRestMapping, nil)

		parsedJsonResult := make(map[string]interface{})
		unstructuredResultMock := &unstructured.Unstructured{Object: parsedJsonResult}

		apiInterfaceMock := newMockNamespaceInterface(t)
		apiInterfaceMock.EXPECT().Patch(mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
			Return(unstructuredResultMock, nil)

		dynClientMock := newMockDynClient(t)
		dynClientMock.EXPECT().Resource(mock.Anything).Return(apiInterfaceMock)

		sut := Applier{
			gvrMapper: gvrMapperMock,
			dynClient: dynClientMock,
		}

		testResource := []byte(`apiVersion: v1
kind: Namespace
metadata:
  name: the-best-resource-in-store
  namespace: ecosystem`)

		// when
		err := sut.Apply(testResource, "mynamespace")

		// then
		require.NoError(t, err)
	})
}

// use this to re-generate newMockNamespaceInterface
type namespaceInterface interface {
	dynamic.NamespaceableResourceInterface
}
