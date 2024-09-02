package routing

import (
	"slices"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

type RouteType string

const (
	PublicRoute   RouteType = "public"
	ExternalRoute RouteType = "external"
)

func AllRouteTypes() []RouteType {
	return []RouteType{PublicRoute, ExternalRoute}
}

func IsValidRouteType(routeType RouteType) bool {
	return slices.Contains(AllRouteTypes(), routeType)
}

func UnusedRouteTypes(exportModes []RouteType) []RouteType {
	used := make(map[RouteType]bool)

	for _, mode := range exportModes {
		used[mode] = true
	}

	var unused []RouteType

	for _, rType := range AllRouteTypes() {
		if !used[rType] {
			unused = append(unused, rType)
		}
	}

	return unused
}

type PlatformRoutingConfiguration struct {
	IngressSelectorLabel,
	IngressSelectorValue,
	IngressService,
	GatewayNamespace string
}

type ExposedServiceConfig struct {
	PlatformRoutingConfiguration
	PublicServiceName,
	ServiceName,
	ServiceNamespace,
	ServiceTargetPort,
	Domain string
}

func NewExposedServiceConfig(config PlatformRoutingConfiguration, svc *corev1.Service, domain string) *ExposedServiceConfig {
	return &ExposedServiceConfig{
		PlatformRoutingConfiguration: config,
		PublicServiceName:            svc.GetName() + "-" + svc.GetNamespace(),
		ServiceName:                  svc.GetName(),
		ServiceNamespace:             svc.GetNamespace(),
		ServiceTargetPort:            svc.Spec.Ports[0].TargetPort.String(),
		Domain:                       domain,
	}
}

// TemplateLoader provides a way to differentiate the Route resource templates used based on its types.
type TemplateLoader interface {
	Load(data *ExposedServiceConfig, routeType RouteType) ([]*unstructured.Unstructured, error)
}
