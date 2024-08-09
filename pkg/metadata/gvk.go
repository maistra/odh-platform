package metadata

import (
	"github.com/opendatahub-io/odh-platform/pkg/spi"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

func ExternalGVKs() []schema.GroupVersionKind {
	return []schema.GroupVersionKind{
		{Group: "route.openshift.io", Version: "v1", Kind: "Route"},
		{Group: "networking.istio.io", Version: "v1beta1", Kind: "VirtualService"},
	}
}

func PublicGVKs() []schema.GroupVersionKind {
	return []schema.GroupVersionKind{
		{Group: "", Version: "v1", Kind: "Service"},
		{Group: "networking.istio.io", Version: "v1beta1", Kind: "Gateway"},
		{Group: "networking.istio.io", Version: "v1beta1", Kind: "VirtualService"},
		{Group: "networking.istio.io", Version: "v1beta1", Kind: "DestinationRule"},
	}
}

func ResourceGVKs(exportMode spi.RouteType) []schema.GroupVersionKind {
	switch exportMode {
	case spi.ExternalRoute:
		return ExternalGVKs()
	case spi.PublicRoute:
		return PublicGVKs()
	}

	return nil
}
