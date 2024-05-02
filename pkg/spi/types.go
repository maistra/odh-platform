package spi

import (
	"context"

	authorinov1beta2 "github.com/kuadrant/authorino/api/v1beta2"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
)

type AuthType string

const (
	UserDefined AuthType = "userdefined"
	Anonymous   AuthType = "anonymous"
)

type AuthorizationComponent struct {
	CustomResourceType schema.GroupVersionKind `json:"gvk"`
	WorkloadSelector   map[string]string       `json:"workloadSelector"` // label key value
	Ports              []string                `json:"ports"`            // port numbers
	HostPaths          []string                `json:"hostPaths"`        // json path expression e.g. status.url
}

// HostExtractor attempts to extract Hosts from the given resource.
type HostExtractor interface {
	Extract(res *unstructured.Unstructured) []string
}

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
