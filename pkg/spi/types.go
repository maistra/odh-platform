package spi

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	authorinov1beta2 "github.com/kuadrant/authorino/api/v1beta2"
	"github.com/opendatahub-io/odh-platform/pkg/metadata/annotations"
	"github.com/opendatahub-io/odh-platform/pkg/platform"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
)

// Auth

type AuthType string

const (
	UserDefined AuthType = "userdefined"
	Anonymous   AuthType = "anonymous"
)

type AuthorizationComponent struct {
	platform.ProtectedResource
}

// TODO: the config file will contain more then just AuthorizationComponents now.. adjust to read it multiple times pr Type or load it all at once..?
// TODO: move the config load and save into a sub package and lazy share with operator.
func (a AuthorizationComponent) Load(configPath string) ([]AuthorizationComponent, error) {
	// TODO(mvp): rework/simplify types
	content, err := os.ReadFile(configPath + string(filepath.Separator) + "authorization")
	if err != nil {
		return []AuthorizationComponent{}, fmt.Errorf("could not read config file [%s]: %w", configPath, err)
	}

	var authz []AuthorizationComponent

	err = json.Unmarshal(content, &authz)
	if err != nil {
		return []AuthorizationComponent{}, fmt.Errorf("could not parse json content of [%s]: %w", configPath, err)
	}

	return authz, nil
}

// HostExtractor attempts to extract Hosts from the given resource.
type HostExtractor func(res *unstructured.Unstructured) ([]string, error)

// AuthTypeDetector attempts to determine the AuthType for the given resource
// Possible implementations might check annotations or labels or possible related objects / config.
type AuthTypeDetector interface {
	Detect(ctx context.Context, res *unstructured.Unstructured) (AuthType, error)
}

// AuthConfigTemplateLoader provides a way to differentiate the AuthConfig template used based on
//   - AuthType
//   - Namespace / Resource name
//   - Loader source
type AuthConfigTemplateLoader interface {
	Load(ctx context.Context, authType AuthType, key types.NamespacedName) (authorinov1beta2.AuthConfig, error)
}

// Routing

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

type RoutingComponent struct {
	platform.RoutingTarget
}

func (r RoutingComponent) Load(configPath string) ([]RoutingComponent, error) {
	content, err := os.ReadFile(configPath + string(filepath.Separator) + "routing")
	if err != nil {
		return []RoutingComponent{}, fmt.Errorf("could not read config file [%s]: %w", configPath, err)
	}

	var routes []RoutingComponent

	err = json.Unmarshal(content, &routes)
	if err != nil {
		return []RoutingComponent{}, fmt.Errorf("could not parse json content of [%s]: %w", configPath, err)
	}

	return routes, nil
}

type PlatformRoutingConfiguration struct {
	IngressSelectorLabel,
	IngressSelectorValue,
	IngressService,
	GatewayNamespace string
}

type RoutingData struct {
	PlatformRoutingConfiguration

	PublicServiceName string // [service-name]-[service-namespace]
	ServiceName       string
	ServiceNamespace  string

	ServiceTargetPort string

	Domain string
}

func NewRoutingData(config PlatformRoutingConfiguration, svc *corev1.Service, domain string) *RoutingData {
	return &RoutingData{
		PlatformRoutingConfiguration: config,
		PublicServiceName:            svc.GetName() + "-" + svc.GetNamespace(),
		ServiceName:                  svc.GetName(),
		ServiceNamespace:             svc.GetNamespace(),
		ServiceTargetPort:            svc.Spec.Ports[0].TargetPort.String(),
		Domain:                       domain,
	}
}

// RoutingTemplateLoader provides a way to differentiate the Route resource templates used based on its types.
type RoutingTemplateLoader interface {
	Load(data *RoutingData, routeType RouteType) ([]*unstructured.Unstructured, error)
}
