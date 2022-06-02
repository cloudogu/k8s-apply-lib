package apply

import (
	"context"
	"fmt"

	"github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer/yaml"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/json"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/discovery/cached/memory"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/restmapper"
	ctrl "sigs.k8s.io/controller-runtime"
)

// YamlDocument is an alias type for exactly one single YAML document
type YamlDocument []byte

type Applier struct {
	gvrMapper meta.RESTMapper
	dynClient dynamic.Interface
	scheme    *runtime.Scheme
}

// New returns a `kubectl`-like apply client which operates on the K8s API with YAML resources.
//
// This method also returns a runtime.Scheme which will be used to properly handle owner references (most important when
// working with your own CRD). Use it like this:
//
//  applier, scheme, err := apply.New(config)
//  yourCrdGroupVersion.AddToScheme(scheme)
func New(clusterConfig *rest.Config) (*Applier, *runtime.Scheme, error) {
	gvrMapper, err := createGVRMapper(clusterConfig)
	if err != nil {
		return nil, nil, fmt.Errorf("error while creating k8s apply client: %w", err)
	}
	dynCli, err := createDynamicClient(clusterConfig)
	if err != nil {
		return nil, nil, fmt.Errorf("error while creating k8s apply client: %w", err)
	}

	schemeForCrdHandling := runtime.NewScheme()

	return &Applier{
			gvrMapper: gvrMapper,
			dynClient: dynCli,
			scheme:    schemeForCrdHandling,
		},
		schemeForCrdHandling,
		nil
}

func createGVRMapper(config *rest.Config) (meta.RESTMapper, error) {
	// 1. Prepare a RESTMapper to find GVR
	dc, err := discovery.NewDiscoveryClientForConfig(config)
	if err != nil {
		return nil, err
	}

	return restmapper.NewDeferredDiscoveryRESTMapper(memory.NewMemCacheClient(dc)), nil
}

func createDynamicClient(config *rest.Config) (dynamic.Interface, error) {
	// 2. Prepare the dynamic client
	dyn, err := dynamic.NewForConfig(config)
	if err != nil {
		return nil, err
	}

	return dyn, nil
}

// Apply sends a request to the K8s API with the provided YAML resource in order to apply them to the current cluster.
func (ac *Applier) Apply(yamlResource YamlDocument, namespace string) error {
	return ac.ApplyWithOwner(yamlResource, namespace, nil)
}

// ApplyWithOwner sends a request to the K8s API with the provided YAML resource in order to apply them to the current cluster.
func (ac *Applier) ApplyWithOwner(yamlResource YamlDocument, namespace string, owningResource metav1.Object) error {
	logrus.Debug("Applying K8s resource")
	logrus.Debug(string(yamlResource))

	// 3. Decode YAML manifest into unstructured.Unstructured
	var decUnstructured = yaml.NewDecodingSerializer(unstructured.UnstructuredJSONScheme)
	k8sObjects := &unstructured.Unstructured{}
	_, gvk, err := decUnstructured.Decode(yamlResource, nil, k8sObjects)
	if err != nil {
		return fmt.Errorf("could not decode YAML doccument '%s': %w", string(yamlResource), err)
	}

	// 4. Map GVK to GVR
	// a resource can be uniquely identified by GroupVersionResource, but we need the GVK to find the corresponding GVR
	gvr, err := ac.gvrMapper.RESTMapping(gvk.GroupKind(), gvk.Version)
	if err != nil {
		return fmt.Errorf("could find GVK mapper for GroupKind=%v,Version=%s and YAML doccument '%s': %w", gvk.GroupKind(), gvk.Version, string(yamlResource), err)
	}

	// 5. Obtain REST interface for the GVR
	var dr dynamic.ResourceInterface
	if gvr.Scope.Name() == meta.RESTScopeNameNamespace {
		k8sObjects.SetNamespace(namespace)
		// namespaced resources should specify the namespace
		dr = ac.dynClient.Resource(gvr.Resource).Namespace(namespace)

		if owningResource != nil {
			err = ctrl.SetControllerReference(owningResource, k8sObjects, ac.scheme)
			if err != nil {
				return fmt.Errorf("could not apply YAML doccument '%s': could not set controller reference: %w", string(yamlResource), err)
			}
		}
	} else {
		// for cluster-wide resources
		dr = ac.dynClient.Resource(gvr.Resource)
	}

	return createOrUpdateResource(context.Background(), k8sObjects, dr)
}

func createOrUpdateResource(ctx context.Context, desiredResource *unstructured.Unstructured, dr dynamic.ResourceInterface) error {
	const fieldManager = "k8s-ces-setup"

	logrus.Debugf("Patching resource %s/%s/%s", desiredResource.GetKind(), desiredResource.GetAPIVersion(), desiredResource.GetName())
	// 6. marshal unstructured resource into proper JSON
	jsondata, err := json.Marshal(desiredResource)
	if err != nil {
		return NewResourceError(err, "error while parsing resource to json", desiredResource.GetKind(), desiredResource.GetAPIVersion(), desiredResource.GetName())
	}

	// 7. Update the object with server-side-apply
	//    types.ApplyPatchType indicates server-side-apply.
	//    FieldManager specifies the field owner ID.
	_, err = dr.Patch(ctx, desiredResource.GetName(), types.ApplyPatchType, jsondata, metav1.PatchOptions{
		FieldManager: fieldManager,
	})
	if err != nil {
		return NewResourceError(err, "error while patching", desiredResource.GetKind(), desiredResource.GetAPIVersion(), desiredResource.GetName())
	}

	return nil
}
