package platform

import "k8s.io/apimachinery/pkg/runtime/schema"

// ObjectReference is a reference to a Kubernetes resource which Platform uses to enable certain capabilities.
// These custom resources serve as single point of configuration for enabling given capability for the component.
type ObjectReference struct {
	// GroupVersionKind specifies the group, version, and kind of the resource.
	schema.GroupVersionKind `json:"gvk,omitempty"`
	// Resources is the type of resource being protected in a plural form, e.g., "pods", "services".
	Resources string `json:"resources,omitempty"`
}

// RoutingTarget represents a target object that routing controller
// will watch to ensure proper routing configuration is created.
type RoutingTarget struct {
	// ObjectReference provides reference details to the associated object.
	ObjectReference `json:"ref,omitempty"`
	// ServiceSelector is a LabelSelector definition to locate the Service(s) to expose to Routing for the given ObjectReference.
	// All provided label selectors must be present on the Service to find a match.
	//
	// go expressions are handled in the selector key and value to set dynamic values from the current ObjectReference;
	// e.g. "routing.opendatahub.io/{{.kind}}": "{{.metadata.name}}", // > "routing.opendatahub.io/Service": "MyService"
	ServiceSelector map[string]string `json:"serviceSelector,omitempty"`
}

// ProtectedResource  holds references and configuration details necessary for
// applying authorization policies to a specific workload.
type ProtectedResource struct {
	// ObjectReference provides reference details to the associated object.
	ObjectReference `json:"ref,omitempty"`
	// WorkloadSelector defines labels used to identify and select the specific workload
	// to which the authorization policy should be applied.
	WorkloadSelector map[string]string `json:"workloadSelector,omitempty"`
	// HostPaths defines paths in custom resource where hosts for this component are defined.
	HostPaths []string `json:"hostPaths,omitempty"` // TODO(mvp): should we switch to annotations like in routing?
	// Ports is a list of network ports associated with the resource that require protection.
	// These ports in conjunction with hosts are subject to the authorization policies defined for the workload.
	Ports []string `json:"ports,omitempty"`
}
