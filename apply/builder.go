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

// PredicatedResourceCollector help to identify and collect specific Kubernetes resources that stream through the
// applier. It is the implementor's task to provide both the predicate to match the resource and to handle the resource
// collection. The collected resources can be fetched after the Applier/Builder finished applying the resources to the
// Kubernetes API.
//
// An example implementation to collect namespace resources might look like this:
//
//	func (c *collector) Predicate(doc YamlDocument) (bool, error) {
//	  var namespace = &v1.Namespace{}
//	  if err := yaml.Unmarshal(doc, namespace); err != nil { return false, err }
//	  return namespace.Kind == "Namespace", nil
//	}
//
//	func (c *collector) Collect(doc YamlDocument) {
//	  c.collected = append(c.collected, doc)
//	}
type PredicatedResourceCollector interface {
	// Predicate returns true if the resource being effectively applied matches against a given predicate.
	Predicate(doc YamlDocument) (bool, error)
	// Collect cumulates all YAML documents that match the predicate over the whole resource application against the
	// Kubernetes API.
	Collect(doc YamlDocument)
}

// ApplyFilter help to filter specific Kubernetes resources that stream through the applier and applies them.
// It is the implementor's task to provide the predicate to match the resource that should not be
// applied. The filtered resources are still collected.
//
// An example implementation to only apply namespace resources could look like this:
//
//	func (c *filter) Predicate(doc YamlDocument) (bool, error) {
//	  var namespace = &v1.Namespace{}
//	  if err := yaml.Unmarshal(doc, namespace); err != nil { return false, err }
//	  return namespace.Kind == "Namespace", nil
//	}
type ApplyFilter interface {
	// Predicate returns true if the resource being effectively applied matches against a given predicate.
	Predicate(doc YamlDocument) (bool, error)
}

// Builder provides a convenience builder that simplifies the Applier usage and adds often-sought features, like
// doc splitting or templating.
//
// Usage:
//
//	 applier, _, err := apply.New(restConfig)
//	 NewBuilder(applier).
//	   WithNamespace("my-namespace").
//		  WithYamlResource(myfile, content).
//		  WithTemplate(myfile, templateObject).
//		  WithYamlResource(myfile2, content2).
//		  WithTemplate(myfile2, templateObject2).
//		  WithApplyFilter(myFilterImplementation).
//	   ExecuteApply()
type Builder struct {
	applier               applier
	fileToGenericResource map[string][]byte
	fileToTemplate        map[string]interface{}
	owningResource        metav1.Object
	namespace             string
	predicatedCollectors  []PredicatedResourceCollector
	applyFilter           ApplyFilter
}

// NewBuilder creates a new builder.
func NewBuilder(applier applier) *Builder {
	return &Builder{
		applier:               applier,
		fileToGenericResource: make(map[string][]byte),
		fileToTemplate:        make(map[string]interface{}),
		predicatedCollectors:  []PredicatedResourceCollector{},
	}
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

// WithCollector adds the given PredicatedResourceCollector to list of collectors. This method is optional.
func (ab *Builder) WithCollector(collector PredicatedResourceCollector) *Builder {
	ab.predicatedCollectors = append(ab.predicatedCollectors, collector)

	return ab
}

// WithApplyFilter set the given ApplyFilter. This method is optional.
// When the applyFilter exists, only resources that match this filter will be applied.
func (ab *Builder) WithApplyFilter(filter ApplyFilter) *Builder {
	ab.applyFilter = filter

	return ab
}

// ExecuteApply executes applies pending template renderings to the cumulated resources, collects resources for any
// configured collectors, and applies the result against the configured Kubernetes API.
func (ab *Builder) ExecuteApply() error {
	err := ab.renderTemplates()
	if err != nil {
		return err
	}

	fileToSingleYamlDocs := ab.splitYamlDocs()

	for filename, yamlDocs := range fileToSingleYamlDocs {
		for _, yamlDoc := range yamlDocs {
			if err := ab.applyDoc(filename, yamlDoc); err != nil {
				return err
			}
		}
	}

	return nil
}

func (ab *Builder) applyDoc(filename string, yamlDoc YamlDocument) error {
	err := ab.runCollectors(yamlDoc)
	if err != nil {
		return fmt.Errorf("resource collection failed for file %s: %w", filename, err)
	}

	if ab.applyFilter != nil {
		ok, err := ab.applyFilter.Predicate(yamlDoc)
		if err != nil {
			return fmt.Errorf("filtering resource failed for file %s: %w", filename, err)
		}

		if !ok {
			// is not filtered -> do not apply
			return nil
		}
	}

	// Use ApplyWithOwner here even if no owner is set because it accepts nil owners
	err = ab.applier.ApplyWithOwner(yamlDoc, ab.namespace, ab.owningResource)
	if err != nil {
		return fmt.Errorf("resource application failed for file %s: %w", filename, err)
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

func (ab *Builder) runCollectors(doc YamlDocument) error {
	for _, predCollector := range ab.predicatedCollectors {
		ok, err := predCollector.Predicate(doc)
		if err != nil {
			return fmt.Errorf("error matching predicate against doc [%s]: %w", string(doc), err)
		}

		if ok {
			predCollector.Collect(doc)
		}
	}

	return nil
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
