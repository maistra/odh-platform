package spi

import (
	"context"

	authorinov1beta2 "github.com/kuadrant/authorino/api/v1beta2"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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
	CustomResourceType schema.GroupVersionKind
	WorkloadSelector   map[string]string // label key value
	Ports              []string          // port numbers
	HostPaths          []string          // json path expression e.g. status.url
}

// TODO: decide on approach
type Component struct {
	// We could enforce a Policy for some key used by Component as part of unified lables
	Name string
	// OR use some sort of
	PolicySelector metav1.LabelSelector

	CustomResourceTypes []schema.GroupVersionKind
}

// HostExtractor attempts to extract Hosts from the given resource
type HostExtractor interface {
	Extract(res *unstructured.Unstructured) []string
}

// AuthTypeDetector attempts to determine the AuthType for the given resource
// Possible implementations might check annotations or labels or possible related objects / config
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
