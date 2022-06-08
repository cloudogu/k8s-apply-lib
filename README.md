# k8s-apply-lib

A library to generically apply Kubernetes resources (similar to `kubectl`).

## Usages

### Basic

```go
func yourCode() {
  yamlBytes := readFile("/your/file.yaml")
  
  applier, _, err := apply.New(yourRestConfig, "your-app-name")
  err := applier.NewBuilder().
    WithNamespace("your-namespace").
    WithYamlResource("/your/file.yaml", doc).
    ExecuteApply()
}
```

### Advanced: Templating included

Often, some data is only available at runtime where `kustomize` does not really cut it. `k8s-apply-lib` provides of course [Go templating](https://golangdocs.com/templates-in-golang). Consider a resource file like this:
```yaml
apiVersion: v1
kind: Namespace
metadata:
  labels:
    something: different
  name: {{ .Namespace }}
---
apiVersion: v1
kind: ServiceAccount
metadata:
  name: another-service-account
```

Just add your templating data with the method `WithTemplating()`, and you're good to go! If you do like so, all occurrences of `.Namespace` in the given file will be rendered and then applied against the cluster.

```go
func yourCode() {
  filename := "/your/fileWithGoTemplating.yaml"
  yamlBytes := readFile(filename)
   templateData := struct {
     Namespace string
   }{ Namespace: "your-namespace" }
   
  applier, _, err := apply.New(yourRestConfig, "your-app-name")
  err := applier.NewBuilder().
    WithNamespace("your-namespace").
    WithYamlResource(filename, doc).
    WithTemplating(filename, templateData).
    ExecuteApply()
}
```

### Advanced: Owner Resources

When working with your own CRDs inside a [Kubernetes Operator](https://kubernetes.io/docs/concepts/extend-kubernetes/operator/) garbage collection is a thing to be taken seriously. `k8s-apply-lib` provides a way of setting an owning resource. This way, if the owning resource is going to be deleted, the applied resources will be deleted as well. Please note, that setting an owner reference works only for namespace-scoped-to-namespace-scoped [ownership relations](https://kubernetes.io/docs/concepts/overview/working-with-objects/owners-dependents/). There can only be one owner per builder run.

```go
func yourCode() {
  filename := "/your/fileWithGoTemplating.yaml"
  yamlBytes := readFile(filename)
  owner := &v1.ServiceAccount{
      TypeMeta: metav1.TypeMeta{
         APIVersion: "v1",
         Kind:       "ServiceAccount",
      },
      ObjectMeta: metav1.ObjectMeta{
         Name:      "le-service-account",
      },
   }
   
  applier, _, err := apply.New(yourRestConfig, "your-app-name")
  err := applier.NewBuilder().
    WithNamespace("your-namespace").
    WithOwner(owner).
    WithYamlResource(filename, doc).
    ExecuteApply()
}
```
### Advanced: Resource Collection

Sometimes a resource being applied to the Kubernetes API needs to be re-used somewhere else (f. i. a ServiceAccount must be mounted by name). `k8s-apply-lib` provides a way of matching and collecting resources while they stream through the `Applier`. `PredicatedResourceCollector` is an interface with two methods which you should implement to collect your resources:
- `Predicate(doc YamlDocument) (bool, error)`
  - should return true if the generic resource in the YAML document should be collected 
- `Collect(doc YamlDocument)`
  - takes care of the actual collection of your liking

Please see the interface `PredicatedResourceCollector` in `Builder.go` for more information.

```go
func yourCode() {
  filename := "/your/file.yaml"
  yamlBytes := readFile(filename)
   
  applier, _, err := apply.New(yourRestConfig, "your-app-name")
  err := applier.NewBuilder().
    WithNamespace("your-namespace").
    WithYamlResource(filename, doc).
    WithCollector(owner).
    ExecuteApply()
}
```

### Advanced: Apply Filter

Sometimes it is required to prevent applying a specific resource contained in a collection of yaml documents. 
`k8s-apply-lib` provides a way of filtering resources before applying them. 
`ApplyFilter` is an interface with one method which you should implement to filter your resources:
- `Predicate(doc YamlDocument) (bool, error)`
  - should return true if the generic resource in the YAML document should be filter, i.e., it should be applied.

Please see the interface `ApplyFilter` in `Builder.go` for more information.

```go
func yourCode() {
  filename := "/your/file.yaml"
  yamlBytes := readFile(filename)
   
  applier, _, err := apply.New(yourRestConfig, "your-app-name")
  err := applier.NewBuilder().
    WithNamespace("your-namespace").
    WithYamlResource(filename, doc).
	WithApplyFilter(myFilterImplementation).
    ExecuteApply()
}
```
---

### What is the Cloudogu EcoSystem?

The Cloudogu EcoSystem is an open platform, which lets you choose how and where your team creates great software. Each
service or tool is delivered as a Dogu, a Docker container. Each Dogu can easily be integrated in your environment just
by pulling it from our registry. We have a growing number of ready-to-use Dogus, e.g. SCM-Manager, Jenkins, Nexus,
SonarQube, Redmine and many more. Every Dogu can be tailored to your specific needs. Take advantage of a central
authentication service, a dynamic navigation, that lets you easily switch between the web UIs and a smart configuration
magic, which automatically detects and responds to dependencies between Dogus. The Cloudogu EcoSystem is open source and
it runs either on-premises or in the cloud. The Cloudogu EcoSystem is developed by Cloudogu GmbH
under [MIT License](https://cloudogu.com/license.html).

### How to get in touch?

Want to talk to the Cloudogu team? Need help or support? There are several ways to get in touch with us:

* [Website](https://cloudogu.com)
* [myCloudogu-Forum](https://forum.cloudogu.com/topic/34?ctx=1)
* [Email hello@cloudogu.com](mailto:hello@cloudogu.com)

---
&copy; 2022 Cloudogu GmbH - MADE WITH :heart:&nbsp;FOR DEV
ADDICTS. [Legal notice / Impressum](https://cloudogu.com/imprint.html)
