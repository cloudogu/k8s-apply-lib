package apply

import (
	"bytes"
	"fmt"
	"text/template"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type applier interface {
	// ApplyWithOwner provides a testable method
	ApplyWithOwner(doc YamlDocument, namespace string, resource metav1.Object) error
}

type Builder struct {
	applier               applier
	fileToGenericResource map[string][]byte
	fileToTemplate        map[string]interface{}
	// owningResource is any resource that keeps a reference on all custom resources for reasons of garbage collection
	owningResource metav1.Object
	namespace      string
}

// WithYamlResource adds another YAML resource to the builder.
func (ab *Builder) WithYamlResource(filename string, yamlResource []byte) *Builder {
	ab.fileToGenericResource[filename] = yamlResource

	return ab
}

// WithTemplate adds templating features to the YAML resource with the given filename. This method is optional.
func (ab *Builder) WithTemplate(filename string, templateObject interface{}) *Builder {
	ab.fileToTemplate[filename] = templateObject

	return ab
}

// WithOwner maintains an owner reference for the YAML resource that should be applied during ExecuteApply. If the
// owning resource is deleted then all associated resources will be deleted as well. This method is optional.
func (ab *Builder) WithOwner(owningResource metav1.Object) *Builder {
	ab.owningResource = owningResource

	return ab
}

// WithNamespace sets the target namespace to which the file's resources will apply. This method is mandatory.
func (ab *Builder) WithNamespace(namespace string) *Builder {
	ab.namespace = namespace

	return ab
}

// ExecuteApply executes applies pending template renderings to the cumulated resources and applies the result against
// the configured Kubernetes API.
func (ab *Builder) ExecuteApply() error {
	err := ab.renderTemplates()
	if err != nil {
		return err
	}

	fileToSingleYamlDocs := ab.splitYamlDocs()

	for filename, yamlDocs := range fileToSingleYamlDocs {
		for _, yamlDoc := range yamlDocs {
			// Use ApplyWithOwner here even if no owner is set because it accepts nil owners
			err = ab.applier.ApplyWithOwner(yamlDoc, ab.namespace, ab.owningResource)
			if err != nil {
				return fmt.Errorf("resource application failed for file %s: %w", filename, err)
			}
		}
	}

	return nil
}

func (ab *Builder) renderTemplates() error {
	if len(ab.fileToTemplate) == 0 {
		return nil
	}

	for filename, resource := range ab.fileToGenericResource {
		templateObject := ab.fileToTemplate[filename]

		transformedResource, err := renderTemplate(filename, resource, templateObject)
		if err != nil {
			return err
		}

		ab.fileToGenericResource[filename] = transformedResource
	}

	return nil
}

func renderTemplate(filename string, templateText []byte, templateObject interface{}) ([]byte, error) {
	const templateName = "t"
	tpl := template.New(templateName)

	parsed, err := tpl.Parse(string(templateText))
	if err != nil {
		return nil, fmt.Errorf("failed to parse template for file %s: %w", filename, err)
	}

	resultWriter := bytes.NewBuffer([]byte{})

	err = parsed.ExecuteTemplate(resultWriter, templateName, templateObject)
	if err != nil {
		return nil, fmt.Errorf("failed to render template for file %s: %w", filename, err)
	}

	return resultWriter.Bytes(), nil
}

func (ab *Builder) splitYamlDocs() map[string][]YamlDocument {
	allSingleYamlDocs := make(map[string][]YamlDocument)
	for filename, resource := range ab.fileToGenericResource {
		yamlDocs := splitResourceIntoDocuments(resource)
		allSingleYamlDocs[filename] = yamlDocs
	}

	return allSingleYamlDocs
}

func splitResourceIntoDocuments(resourceBytes []byte) []YamlDocument {
	yamlFileSeparator := []byte("---\n")

	preResult := bytes.Split(resourceBytes, yamlFileSeparator)

	cleanedResult := make([]YamlDocument, 0)
	for _, section := range preResult {
		if len(section) > 0 {
			cleanedResult = append(cleanedResult, section)
		}
	}

	return cleanedResult
}
