package authorization

import (
	"context"

	"github.com/kuadrant/authorino/api/v1beta2"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// ProviderConfig holds the configuration for the authorization component as defined by the platform.
type ProviderConfig struct {
	// Label in a format of key=value. It's used to target created AuthConfig by Authorino instance.
	Label string
	// Audiences is a list of audiences used in the AuthConfig template when performing TokenReview.
	Audiences []string
	// ProviderName is the name of the registered external authorization provider in Service Mesh.
	ProviderName string
}

// AuthType represents the type of authentication to be used for a given resource.
type AuthType string

const (
	UserDefined AuthType = "userdefined"
	Anonymous   AuthType = "anonymous"
)

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
	Load(ctx context.Context, authType AuthType, templateData map[string]any) (v1beta2.AuthConfig, error)
}
