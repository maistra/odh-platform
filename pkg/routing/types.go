package routing

import (
	"strings"

	"github.com/opendatahub-io/odh-platform/pkg/metadata/annotations"
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

func IsValidRouteType(annotationKey string) (RouteType, bool) {
	if !strings.HasPrefix(annotationKey, annotations.RoutingExportModePrefix) {
		return "", false
	}

	routeType := RouteType(strings.TrimPrefix(annotationKey, annotations.RoutingExportModePrefix))

	for _, validType := range AllRouteTypes() {
		if routeType == validType {
			return routeType, true
		}
	}

	return routeType, false
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

// IngressConfig holds the configuration for the ingress resources (Istio Ingress Gateway services).
// These values determine how and where additional resources required for platform routing will be created.
type IngressConfig struct {
	IngressSelectorLabel,
	IngressSelectorValue,
	IngressService,
	GatewayNamespace string
}

// ExposedServiceConfig holds the configuration for a service that is used to serve as a cluster-local service facade
// allowing non-mesh clients to access mesh services.
type ExposedServiceConfig struct {
	IngressConfig
	PublicServiceName,
	ServiceName,
	ServiceNamespace,
	ServiceTargetPort,
	Domain string
}

func NewExposedServiceConfig(svc *corev1.Service, config IngressConfig, domain string) *ExposedServiceConfig {
	return &ExposedServiceConfig{
		IngressConfig:     config,
		PublicServiceName: svc.GetName() + "-" + svc.GetNamespace(),
		ServiceName:       svc.GetName(),
		ServiceNamespace:  svc.GetNamespace(),
		ServiceTargetPort: svc.Spec.Ports[0].TargetPort.String(),
		Domain:            domain,
	}
}

// TemplateLoader provides a way to differentiate the Route resource templates used based on its types.
type TemplateLoader interface {
	Load(data *ExposedServiceConfig, routeType RouteType) ([]*unstructured.Unstructured, error)
}
