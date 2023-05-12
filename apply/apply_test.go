package apply

import (
	v1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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
	t.Run("should fail for creating GVR mapper", func(t *testing.T) {
		_, _, err := New(&rest.Config{
			Host: "unparsableHost\\9000",
		}, testFieldManagerName)

		require.Error(t, err)
		assert.Contains(t, err.Error(), "error while creating GVR mapper")
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

	t.Run("should fail for invalid yaml", func(t *testing.T) {
		// given
		sut := Applier{
			gvrMapper: nil,
			dynClient: nil,
		}

		testResource := []byte(`invalid YAML`)

		// when
		err := sut.Apply(testResource, "mynamespace")

		// then
		require.Error(t, err)
		assert.ErrorContains(t, err, "could not decode YAML document")
	})

	t.Run("should fail to create RESTMapping for resource", func(t *testing.T) {
		// given
		expectedResourceGroupKind := schema.GroupKind{Group: "", Kind: "Namespace"}
		gvrMapperMock := newMockGvrMapper(t)
		gvrMapperMock.EXPECT().RESTMapping(expectedResourceGroupKind, "v1").Return(nil, assert.AnError)

		sut := Applier{
			gvrMapper: gvrMapperMock,
			dynClient: nil,
		}

		testResource := []byte(`apiVersion: v1
kind: Namespace
metadata:
  name: the-best-resource-in-store
  namespace: ecosystem`)

		// when
		err := sut.Apply(testResource, "mynamespace")

		// then
		require.Error(t, err)
		assert.ErrorIs(t, err, assert.AnError)
		assert.ErrorContains(t, err, "could not find GVK mapper for GroupKind=Namespace,Version=v1 and YAML document")
	})
}

func Test_Applier_ApplyWithOwner(t *testing.T) {
	t.Run("should fail to set controller reference", func(t *testing.T) {
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
			Scope: meta.RESTScopeNamespace,
		}
		gvrMapperMock := newMockGvrMapper(t)
		gvrMapperMock.EXPECT().RESTMapping(expectedResourceGroupKind, "v1").Return(mockedRestMapping, nil)

		apiInterfaceMock := newMockNamespaceInterface(t)
		apiInterfaceMock.EXPECT().Namespace("mynamespace").Return(nil)

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

		owningResource := &v1.Deployment{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: "otherNamespace",
			},
		}

		// when
		err := sut.ApplyWithOwner(testResource, "mynamespace", owningResource)

		// then
		require.Error(t, err)
		assert.ErrorContains(t, err, "could not apply YAML document")
		assert.ErrorContains(t, err, "could not set controller reference")
	})

	t.Run("should fail to PATCH resource", func(t *testing.T) {
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

		apiInterfaceMock := newMockNamespaceInterface(t)
		apiInterfaceMock.EXPECT().Patch(mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
			Return(nil, assert.AnError)

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
		require.Error(t, err)
		assert.ErrorIs(t, err, assert.AnError)
		assert.ErrorContains(t, err, "error while patching")
	})
}

// use this to re-generate newMockNamespaceInterface
type namespaceInterface interface {
	dynamic.NamespaceableResourceInterface
}
