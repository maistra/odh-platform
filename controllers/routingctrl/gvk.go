package routingctrl

import (
	"github.com/opendatahub-io/odh-platform/pkg/routing"
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

func routingResourceGVKs(exportModes ...routing.RouteType) []schema.GroupVersionKind {
	// use map just to handle possible duplication of gvks
	gvkSet := make(map[schema.GroupVersionKind]struct{})

	for _, exportMode := range exportModes {
		var gvks []schema.GroupVersionKind

		switch exportMode {
		case routing.ExternalRoute:
			gvks = externalGVKs
		case routing.PublicRoute:
			gvks = publicGVKs
		}

		for _, gvk := range gvks {
			gvkSet[gvk] = struct{}{}
		}
	}

	result := make([]schema.GroupVersionKind, 0, len(gvkSet))
	for gvk := range gvkSet {
		result = append(result, gvk)
	}

	return result
}
