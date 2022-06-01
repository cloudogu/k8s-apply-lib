package apply

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/rest"
)

func TestNew(t *testing.T) {
	actual, scheme, _ := New(&rest.Config{})

	require.NotNil(t, actual)
	assert.NotNil(t, scheme)
}

func Test_Applier_implements_interface(t *testing.T) {
	sut, _, err := New(&rest.Config{})

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
		gvrMapperMock := &mockGvrMapper{}
		gvrMapperMock.On("RESTMapping", expectedResourceGroupKind, []string{"v1"}).Return(mockedRestMapping, nil)

		parsedJsonResult := make(map[string]interface{})
		unstructuredResultMock := &unstructured.Unstructured{Object: parsedJsonResult}

		apiInterfaceMock := &mockNsResourceInterface{}
		apiInterfaceMock.On("Patch", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
			Return(unstructuredResultMock, nil)

		dynClientMock := &mockDynClient{}
		dynClientMock.On("Resource", mock.Anything).Return(apiInterfaceMock)

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
		gvrMapperMock.AssertExpectations(t)
		dynClientMock.AssertExpectations(t)
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
		gvrMapperMock := &mockGvrMapper{}
		gvrMapperMock.On("RESTMapping", expectedResourceGroupKind, []string{"v1"}).Return(mockedRestMapping, nil)

		parsedJsonResult := make(map[string]interface{})
		unstructuredResultMock := &unstructured.Unstructured{Object: parsedJsonResult}

		apiInterfaceMock := &mockNsResourceInterface{}
		apiInterfaceMock.On("Patch", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
			Return(unstructuredResultMock, nil)

		dynClientMock := &mockDynClient{}
		dynClientMock.On("Resource", mock.Anything).Return(apiInterfaceMock)

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
		gvrMapperMock.AssertExpectations(t)
		dynClientMock.AssertExpectations(t)
	})
}

func TestApplier_NewBuilder(t *testing.T) {
	t.Run("should return builder with valid values", func(t *testing.T) {
		sut, _, _ := New(&rest.Config{})

		// when
		builder := sut.NewBuilder()
		// then
		assert.NotNil(t, builder)
		assert.NotNil(t, builder.applier)
		assert.NotNil(t, builder.fileToGenericResource)
		assert.NotNil(t, builder.fileToTemplate)
	})
}

type mockGvrMapper struct {
	mock.Mock
}

func (m *mockGvrMapper) RESTMapping(gk schema.GroupKind, versions ...string) (*meta.RESTMapping, error) {
	args := m.Called(gk, versions)
	return args.Get(0).(*meta.RESTMapping), args.Error(1)
}

func (m *mockGvrMapper) KindFor(_ schema.GroupVersionResource) (schema.GroupVersionKind, error) {
	panic("implement me")
}

func (m *mockGvrMapper) KindsFor(_ schema.GroupVersionResource) ([]schema.GroupVersionKind, error) {
	panic("implement me")
}

func (m *mockGvrMapper) ResourceFor(_ schema.GroupVersionResource) (schema.GroupVersionResource, error) {
	panic("implement me")
}

func (m *mockGvrMapper) ResourcesFor(_ schema.GroupVersionResource) ([]schema.GroupVersionResource, error) {
	panic("implement me")
}

func (m *mockGvrMapper) RESTMappings(_ schema.GroupKind, _ ...string) ([]*meta.RESTMapping, error) {
	panic("implement me")
}

func (m *mockGvrMapper) ResourceSingularizer(_ string) (singular string, err error) {
	panic("implement me")
}

type mockDynClient struct {
	mock.Mock
}

func (m *mockDynClient) Resource(resource schema.GroupVersionResource) dynamic.NamespaceableResourceInterface {
	args := m.Called(resource)
	return args.Get(0).(dynamic.NamespaceableResourceInterface)
}

type mockNsResourceInterface struct {
	mock.Mock
}

func (m *mockNsResourceInterface) Namespace(_ string) dynamic.ResourceInterface {
	return m
}

func (m *mockNsResourceInterface) Patch(ctx context.Context, name string, pt types.PatchType, data []byte, options metav1.PatchOptions, subresources ...string) (*unstructured.Unstructured, error) {
	args := m.Called(ctx, name, pt, data, options, subresources)
	return args.Get(0).(*unstructured.Unstructured), args.Error(1)
}

func (m *mockNsResourceInterface) Create(_ context.Context, _ *unstructured.Unstructured, _ metav1.CreateOptions, _ ...string) (*unstructured.Unstructured, error) {
	panic("implement me")
}

func (m *mockNsResourceInterface) Update(_ context.Context, _ *unstructured.Unstructured, _ metav1.UpdateOptions, _ ...string) (*unstructured.Unstructured, error) {
	panic("implement me")
}

func (m *mockNsResourceInterface) UpdateStatus(_ context.Context, _ *unstructured.Unstructured, _ metav1.UpdateOptions) (*unstructured.Unstructured, error) {
	panic("implement me")
}

func (m *mockNsResourceInterface) Delete(_ context.Context, _ string, _ metav1.DeleteOptions, _ ...string) error {
	panic("implement me")
}

func (m *mockNsResourceInterface) DeleteCollection(_ context.Context, _ metav1.DeleteOptions, _ metav1.ListOptions) error {
	panic("implement me")
}

func (m *mockNsResourceInterface) Get(_ context.Context, _ string, _ metav1.GetOptions, _ ...string) (*unstructured.Unstructured, error) {
	panic("implement me")
}

func (m *mockNsResourceInterface) List(_ context.Context, _ metav1.ListOptions) (*unstructured.UnstructuredList, error) {
	panic("implement me")
}

func (m *mockNsResourceInterface) Watch(_ context.Context, _ metav1.ListOptions) (watch.Interface, error) {
	panic("implement me")
}
