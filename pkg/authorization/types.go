package authorization

import (
	"context"

	"github.com/kuadrant/authorino/api/v1beta2"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
)

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
	Load(ctx context.Context, authType AuthType, key types.NamespacedName) (v1beta2.AuthConfig, error)
}
