package routing

import (
	"github.com/opendatahub-io/odh-platform/pkg/spi"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

//nolint:gochecknoglobals // reason: externalGVKs is a static list of GVKs that doesn't need to be generated
var externalGVKs = []schema.GroupVersionKind{
	{Group: "route.openshift.io", Version: "v1", Kind: "Route"},
	{Group: "networking.istio.io", Version: "v1beta1", Kind: "VirtualService"},
}

//nolint:gochecknoglobals // reason: publicGVKs is a static list of GVKs that doesn't need to be generated
var publicGVKs = []schema.GroupVersionKind{
	{Group: "", Version: "v1", Kind: "Service"},
	{Group: "networking.istio.io", Version: "v1beta1", Kind: "Gateway"},
	{Group: "networking.istio.io", Version: "v1beta1", Kind: "VirtualService"},
	{Group: "networking.istio.io", Version: "v1beta1", Kind: "DestinationRule"},
}

func routingResourceGVKs(exportMode spi.RouteType) []schema.GroupVersionKind {
	switch exportMode {
	case spi.ExternalRoute:
		return externalGVKs
	case spi.PublicRoute:
		return publicGVKs
	}

	return nil
}
