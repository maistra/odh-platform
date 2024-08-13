package spi

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	authorinov1beta2 "github.com/kuadrant/authorino/api/v1beta2"
	"github.com/opendatahub-io/odh-platform/pkg/platform"
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

type PlatformAuthorizationConfig struct {
	// Label in a format of key=value. It's used to target created AuthConfig by Authorino instance.
	Label string
	// Audiences is a list of audiences that will be used in the AuthConfig template when performing TokenReview.
	Audiences []string
	// ProviderName is the name of the registered external authorization provider in Service Mesh.
	ProviderName string
}

// TODO: the config file will contain more then just AuthorizationComponents now.. adjust to read it multiple times pr Type or load it all at once..?
// TODO: move the config load and save into a sub package and lazy share with operator.
func (a AuthorizationComponent) Load(configPath string) ([]AuthorizationComponent, error) {
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
type HostExtractor interface {
	Extract(res *unstructured.Unstructured) ([]string, error)
}

// AuthTypeDetector attempts to determine the AuthType for the given resource
// Possible implementations might check annotations or labels or possible related objects / config.
type AuthTypeDetector interface {
	Detect(ctx context.Context, res *unstructured.Unstructured) (AuthType, error)
}

// Routing

// AuthConfigTemplateLoader provides a way to differentiate the AuthConfig template used based on
//   - AuthType
//   - Namespace / Resource name
//   - Loader source
type AuthConfigTemplateLoader interface {
	Load(ctx context.Context, authType AuthType, key types.NamespacedName) (authorinov1beta2.AuthConfig, error)
}

type RouteType string

const (
	PublicRoute   RouteType = "public"
	ExternalRoute RouteType = "external"
)

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

// RoutingTemplateData is the data required to render a routing template.
type RoutingTemplateData struct { // TODO(mvp): revise the stuct name - is it only for templates?
	PlatformRoutingConfiguration

	PublicServiceName string // [service-name]-[service-namespace]
	ServiceName       string
	ServiceNamespace  string

	ServiceTargetPort string

	Domain string
}

// RoutingTemplateLoader provides a way to differentiate the Route template used based on
//   - RouteType
//   - Namespace / Resource name
//   - Loader source
type RoutingTemplateLoader interface {
	Load(ctx context.Context, routeType RouteType, key types.NamespacedName, data RoutingTemplateData) ([]*unstructured.Unstructured, error)
}
