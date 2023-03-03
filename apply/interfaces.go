package apply

import (
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/client-go/dynamic"
)

type gvrMapper interface {
	meta.RESTMapper
}
type dynClient interface {
	dynamic.Interface
}
