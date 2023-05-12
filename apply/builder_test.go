package apply

import (
	_ "embed"
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/yaml"
)

const (
	testFile1     = "/dir/file1.yaml"
	testFile2     = "/dir/file2.yaml"
	testNamespace = "le-namespace"
)

//go:embed testdata/single-doc.yaml
var singleDocYamlBytes []byte

//go:embed testdata/multi-doc.yaml
var multiDocYamlBytes []byte

//go:embed testdata/multi-doc-template.yaml
var multiDocYamlTemplateBytes []byte

func Test_NewBuilder(t *testing.T) {
	t.Run("should return builder with valid values", func(t *testing.T) {
		applier := &mockApplier{}

		// when
		builder := NewBuilder(applier)
		// then
		assert.NotNil(t, builder)
		assert.NotNil(t, builder.applier)
		assert.NotNil(t, builder.fileToGenericResource)
		assert.NotNil(t, builder.fileToTemplate)
	})
}

func TestBuilder_WithYamlResource(t *testing.T) {
	t.Run("should add a single resource", func(t *testing.T) {
		sut := NewBuilder(nil)

		// when
		sut.WithYamlResource(testFile1, multiDocYamlBytes)

		// then
		assert.NotEmpty(t, sut.fileToGenericResource[testFile1])
		assert.Equal(t, multiDocYamlBytes, sut.fileToGenericResource[testFile1])
	})
	t.Run("should distinguish between different files", func(t *testing.T) {
		sut := NewBuilder(nil)

		// when
		sut.WithYamlResource(testFile1, multiDocYamlBytes).
			WithYamlResource(testFile2, multiDocYamlTemplateBytes)

		// then
		require.Len(t, sut.fileToGenericResource, 2)

		assert.NotEmpty(t, sut.fileToGenericResource[testFile1])
		assert.Equal(t, multiDocYamlBytes, sut.fileToGenericResource[testFile1])

		assert.NotEmpty(t, sut.fileToGenericResource[testFile2])
		assert.Equal(t, multiDocYamlTemplateBytes, sut.fileToGenericResource[testFile2])
	})
}

func TestBuilder_WithTemplate(t *testing.T) {
	t.Run("should add a single template", func(t *testing.T) {
		sut := NewBuilder(nil)
		templateObj := struct {
			Namespace string
		}{
			Namespace: testNamespace,
		}

		// when
		sut.WithTemplate(testFile2, templateObj)

		// then
		assert.NotEmpty(t, sut.fileToTemplate[testFile2])
		assert.Equal(t, templateObj, sut.fileToTemplate[testFile2])
	})
	t.Run("should maintain two different template objects", func(t *testing.T) {
		sut := NewBuilder(nil)
		templateObj1 := struct {
			Namespace string
		}{
			Namespace: testNamespace,
		}
		templateObj2 := struct {
			Namespace string
		}{
			Namespace: "hello-world",
		}

		// when
		sut.WithTemplate(testFile1, templateObj1).
			WithTemplate(testFile2, templateObj2)

		// then
		require.Len(t, sut.fileToTemplate, 2)
		assert.NotEmpty(t, sut.fileToTemplate[testFile1])
		assert.Equal(t, templateObj1, sut.fileToTemplate[testFile1])

		assert.NotEmpty(t, sut.fileToTemplate[testFile2])
		assert.Equal(t, templateObj2, sut.fileToTemplate[testFile2])
	})
}

func TestBuilder_WithOwner(t *testing.T) {
	t.Run("should add an owner resource for all generic resources", func(t *testing.T) {
		sut := NewBuilder(nil)
		anyObject := &v1.ServiceAccount{
			TypeMeta: metav1.TypeMeta{
				APIVersion: "v1",
				Kind:       "ServiceAccount",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      "le-service-account",
				Namespace: testNamespace,
			},
		}

		// when
		sut.WithOwner(anyObject)

		// then
		assert.NotNil(t, sut.owningResource)
		assert.Equal(t, anyObject, sut.owningResource)
	})
}

func TestBuilder_WithCollector(t *testing.T) {
	t.Run("should add an owner resource for all generic resources", func(t *testing.T) {
		sut := NewBuilder(nil)

		collector := &predicatedNamespaceCollector{}

		// when
		sut.WithCollector(collector)

		// then
		assert.NotNil(t, sut.predicatedCollectors)
		assert.Len(t, sut.predicatedCollectors, 1)
		assert.Same(t, collector, sut.predicatedCollectors[0])
	})
}

func TestBuilder_WithApplyFilter(t *testing.T) {
	t.Run("should add an apply filter resource for all generic resources", func(t *testing.T) {
		sut := NewBuilder(nil)

		filter := &predicatedNamespaceCollector{}

		// when
		sut.WithApplyFilter(filter)

		// then
		assert.NotNil(t, sut.applyFilter)
		assert.Same(t, filter, sut.applyFilter)
	})
}

func Test_renderTemplate(t *testing.T) {
	t.Run("should template namespace", func(t *testing.T) {
		tempDoc := []byte(`hello {{ .Namespace }}`)
		templateObj1 := struct {
			Namespace string
		}{
			Namespace: testNamespace,
		}

		actual, err := renderTemplate(testFile1, tempDoc, templateObj1)

		require.NoError(t, err)
		expected := []byte(`hello le-namespace`)
		assert.Equal(t, expected, actual)
	})

	t.Run("should return error", func(t *testing.T) {
		tempDoc := []byte(`hello {{ .Namespace `)
		templateObj1 := struct {
			Namespace string
		}{
			Namespace: testNamespace,
		}

		_, err := renderTemplate(testFile1, tempDoc, templateObj1)

		require.Error(t, err)
		assert.Equal(t, "failed to parse template for file /dir/file1.yaml: template: t:1: unclosed action", err.Error())
	})

	t.Run("should fail to render template", func(t *testing.T) {
		tempDoc := []byte(`hello {{ .Namespace }}`)
		templateObj1 := struct{}{}

		_, err := renderTemplate(testFile1, tempDoc, templateObj1)

		require.Error(t, err)
		assert.ErrorContains(t, err, "failed to render template for file /dir/file1.yaml")
	})
}

func TestBuilder_ExecuteApply(t *testing.T) {
	t.Run("should apply a simple file resource", func(t *testing.T) {
		// given
		doc1 := YamlDocument(singleDocYamlBytes)
		mockedApplier := &mockApplier{}
		mockedApplier.On("ApplyWithOwner", doc1, testNamespace, nil).Return(nil)

		sut := NewBuilder(mockedApplier)

		// when
		err := sut.WithNamespace(testNamespace).
			WithYamlResource(testFile1, doc1).
			ExecuteApply()

		// then
		require.NoError(t, err)
		mockedApplier.AssertExpectations(t)
	})
	t.Run("should apply file resource with owner", func(t *testing.T) {
		// given
		owner := &v1.ConfigMap{
			TypeMeta: metav1.TypeMeta{
				APIVersion: "v1",
				Kind:       "Configmap",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      "le-config-map",
				Namespace: testNamespace,
			},
		}
		doc1 := YamlDocument(singleDocYamlBytes)
		mockedApplier := &mockApplier{}
		mockedApplier.On("ApplyWithOwner", doc1, testNamespace, owner).Return(nil)

		sut := NewBuilder(mockedApplier)

		// when
		err := sut.WithNamespace(testNamespace).
			WithOwner(owner).
			WithYamlResource(testFile1, doc1).
			ExecuteApply()

		// then
		require.NoError(t, err)
		mockedApplier.AssertExpectations(t)
	})
	t.Run("should apply multi doc file resource with template object", func(t *testing.T) {
		// given
		expectedNamespaceDoc := YamlDocument(`apiVersion: v1
kind: Namespace
metadata:
  labels:
    something: different
  name: le-namespace
`)
		expectedServiceAccountDoc := YamlDocument(`apiVersion: v1
kind: ServiceAccount
metadata:
  name: another-service-account
`)
		mockedApplier := &mockApplier{}
		mockedApplier.On("ApplyWithOwner", expectedNamespaceDoc, testNamespace, nil).Return(nil)
		mockedApplier.On("ApplyWithOwner", expectedServiceAccountDoc, testNamespace, nil).Return(nil)

		sut := NewBuilder(mockedApplier)
		doc := YamlDocument(multiDocYamlTemplateBytes)
		templateObj := struct {
			Namespace string
		}{
			Namespace: testNamespace,
		}

		// when
		err := sut.WithNamespace(testNamespace).
			WithYamlResource(testFile2, doc).
			WithTemplate(testFile2, templateObj).
			ExecuteApply()

		// then
		require.NoError(t, err)
		mockedApplier.AssertExpectations(t)
	})
	t.Run("should apply multi doc file resource with filter for service accounts", func(t *testing.T) {
		// given
		expectedServiceAccountDoc := YamlDocument(`apiVersion: v1
kind: ServiceAccount
metadata:
  name: another-service-account
`)
		mockedApplier := &mockApplier{}
		mockedApplier.On("ApplyWithOwner", expectedServiceAccountDoc, testNamespace, nil).Return(nil)

		sut := NewBuilder(mockedApplier)
		doc := YamlDocument(multiDocYamlTemplateBytes)
		templateObj := struct {
			Namespace string
		}{
			Namespace: testNamespace,
		}

		// when
		err := sut.WithNamespace(testNamespace).
			WithYamlResource(testFile2, doc).
			WithTemplate(testFile2, templateObj).
			WithApplyFilter(&predicatedServiceAccountCollector{}).
			ExecuteApply()

		// then
		require.NoError(t, err)
		mockedApplier.AssertExpectations(t)
	})
	t.Run("should collect a single matching resource with two different predicate collectors", func(t *testing.T) {
		// given
		expectedNamespaceDoc := YamlDocument(`apiVersion: v1
kind: Namespace
metadata:
  labels:
    something: different
  name: le-namespace
`)
		expectedServiceAccountDoc := YamlDocument(`apiVersion: v1
kind: ServiceAccount
metadata:
  name: another-service-account
`)
		mockedApplier := &mockApplier{}
		mockedApplier.On("ApplyWithOwner", expectedNamespaceDoc, testNamespace, nil).Return(nil)
		mockedApplier.On("ApplyWithOwner", expectedServiceAccountDoc, testNamespace, nil).Return(nil)

		sut := NewBuilder(mockedApplier)

		doc := YamlDocument(multiDocYamlTemplateBytes)
		templateObj := struct {
			Namespace string
		}{
			Namespace: testNamespace,
		}
		nsCollector := &predicatedNamespaceCollector{}
		saCollector := &predicatedServiceAccountCollector{}

		// when
		err := sut.WithNamespace(testNamespace).
			WithYamlResource(testFile2, doc).
			WithTemplate(testFile2, templateObj).
			WithCollector(nsCollector).
			WithCollector(saCollector).
			ExecuteApply()

		// then
		require.NoError(t, err)
		require.NotEmpty(t, nsCollector.collected)
		assert.Len(t, nsCollector.collected, 1)
		assert.Equal(t, expectedNamespaceDoc, nsCollector.collected[0])
		assert.Len(t, saCollector.collected, 1)
		assert.Equal(t, expectedServiceAccountDoc, saCollector.collected[0])
		mockedApplier.AssertExpectations(t)
	})
	t.Run("should fail to render templates", func(t *testing.T) {
		// given
		doc1 := YamlDocument("Invalid template {{.foo}")
		mockedApplier := &mockApplier{}
		mockedApplier.On("ApplyWithOwner", doc1, testNamespace, nil).Return(nil)

		sut := NewBuilder(mockedApplier).WithYamlResource(testFile1, doc1).WithTemplate(testFile1, doc1)

		// when
		err := sut.WithNamespace(testNamespace).
			WithYamlResource(testFile1, doc1).
			ExecuteApply()

		// then
		require.Error(t, err)
		assert.ErrorContains(t, err, "failed to parse template for file /dir/file1.yaml")
	})
	t.Run("should fail to run collectors", func(t *testing.T) {
		// given
		owner := &v1.ConfigMap{
			TypeMeta: metav1.TypeMeta{
				APIVersion: "v1",
				Kind:       "Configmap",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      "le-config-map",
				Namespace: testNamespace,
			},
		}
		doc1 := YamlDocument(singleDocYamlBytes)
		mockedApplier := &mockApplier{}
		mockedApplier.On("ApplyWithOwner", doc1, testNamespace, owner).Return(nil)

		sut := NewBuilder(mockedApplier)

		// when
		err := sut.WithNamespace(testNamespace).
			WithOwner(owner).
			WithYamlResource(testFile1, doc1).
			WithCollector(&failingPredicateCollector{err: assert.AnError}).
			ExecuteApply()

		// then
		require.Error(t, err)
		assert.ErrorIs(t, errors.Unwrap(err), assert.AnError)
		assert.ErrorContains(t, err, "resource collection failed for file")
	})
	t.Run("should fail to apply filter", func(t *testing.T) {
		// given
		owner := &v1.ConfigMap{
			TypeMeta: metav1.TypeMeta{
				APIVersion: "v1",
				Kind:       "Configmap",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      "le-config-map",
				Namespace: testNamespace,
			},
		}
		doc1 := YamlDocument(singleDocYamlBytes)
		mockedApplier := &mockApplier{}
		mockedApplier.On("ApplyWithOwner", doc1, testNamespace, owner).Return(nil)

		sut := NewBuilder(mockedApplier)

		// when
		err := sut.WithNamespace(testNamespace).
			WithOwner(owner).
			WithYamlResource(testFile1, doc1).
			WithApplyFilter(&failingPredicateCollector{err: assert.AnError}).
			ExecuteApply()

		// then
		require.Error(t, err)
		assert.ErrorIs(t, errors.Unwrap(err), assert.AnError)
		assert.ErrorContains(t, err, "filtering resource failed for file")
	})
	t.Run("should fail to apply with owner", func(t *testing.T) {
		// given
		owner := &v1.ConfigMap{
			TypeMeta: metav1.TypeMeta{
				APIVersion: "v1",
				Kind:       "Configmap",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      "le-config-map",
				Namespace: testNamespace,
			},
		}
		doc1 := YamlDocument(singleDocYamlBytes)
		mockedApplier := &mockApplier{}
		mockedApplier.On("ApplyWithOwner", doc1, testNamespace, owner).Return(assert.AnError)

		sut := NewBuilder(mockedApplier)

		// when
		err := sut.WithNamespace(testNamespace).
			WithOwner(owner).
			WithYamlResource(testFile1, doc1).
			ExecuteApply()

		// then
		require.Error(t, err)
		assert.ErrorIs(t, errors.Unwrap(err), assert.AnError)
		assert.ErrorContains(t, err, "resource application failed for file")
	})
}

type predicatedNamespaceCollector struct {
	collected []YamlDocument
}

func (nsc *predicatedNamespaceCollector) Predicate(doc YamlDocument) (bool, error) {
	var namespace = &v1.Namespace{}

	err := yaml.Unmarshal(doc, namespace)
	if err != nil {
		return false, fmt.Errorf("failed to unmarshal object [%s] into resource kind %s: %w",
			string(doc), namespace.Kind, err)
	}

	return namespace.Kind == "Namespace", nil
}

func (nsc *predicatedNamespaceCollector) Collect(doc YamlDocument) {
	if nsc.collected == nil {
		nsc.collected = []YamlDocument{}
	}

	nsc.collected = append(nsc.collected, doc)
}

type predicatedServiceAccountCollector struct {
	collected []YamlDocument
}

func (sac *predicatedServiceAccountCollector) Predicate(doc YamlDocument) (bool, error) {
	var sa = &v1.ServiceAccount{}

	err := yaml.Unmarshal(doc, sa)
	if err != nil {
		return false, fmt.Errorf("failed to unmarshal object [%s] into resource kind %s: %w",
			string(doc), sa.Kind, err)
	}

	return sa.Kind == "ServiceAccount", nil
}

func (sac *predicatedServiceAccountCollector) Collect(doc YamlDocument) {
	if sac.collected == nil {
		sac.collected = []YamlDocument{}
	}

	sac.collected = append(sac.collected, doc)
}

type failingPredicateCollector struct {
	err error
}

func (fpc *failingPredicateCollector) Predicate(YamlDocument) (bool, error) {
	return false, fpc.err
}

func (fpc *failingPredicateCollector) Collect(YamlDocument) {}

type mockApplier struct {
	mock.Mock
}

func (m *mockApplier) ApplyWithOwner(doc YamlDocument, namespace string, resource metav1.Object) error {
	args := m.Called(doc, namespace, resource)
	return args.Error(0)
}
