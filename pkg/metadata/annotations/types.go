package annotations

import (
	"github.com/opendatahub-io/odh-platform/pkg/metadata"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	RoutingExportModePrefix = "routing.opendatahub.io/export-mode-"
)

type Annotation interface {
	metadata.Option
	metadata.KeyValue
}

// AuthEnabled is an Annotation to enroll given component to authentication
// and authorization framework provided by Opendatahub Platform. It is used
// on the component's Custom Resource which is watched by Platform's controller.
type AuthEnabled string

func (a AuthEnabled) ApplyToMeta(obj metav1.Object) {
	addAnnotation(a, obj)
}

func (a AuthEnabled) Key() string {
	return "security.opendatahub.io/enable-auth"
}

func (a AuthEnabled) Value() string {
	return string(a)
}

// AuthorizationGroup defines the group given Authorization configuration belongs to.
// It is used on Platform's AuthConfig to indicate which Authorization service should
// be handling the configuration.
type AuthorizationGroup string

func (a AuthorizationGroup) ApplyToMeta(obj metav1.Object) {
	addAnnotation(a, obj)
}

func (a AuthorizationGroup) Key() string {
	return "security.opendatahub.io/authorization-group"
}

func (a AuthorizationGroup) Value() string {
	return string(a)
}

// RoutingExportMode defines an individual export mode for the routing capability.
// Each mode (currently: "public" or "external") is represented by a separate annotation.
// The annotation key is formed by prefixing the mode with "routing.opendatahub.io/export-mode-", value is boolean.
type RoutingExportMode struct {
	mode  string
	value string
}

func ExternalMode() RoutingExportMode {
	return RoutingExportMode{mode: "external", value: "true"}
}

func PublicMode() RoutingExportMode {
	return RoutingExportMode{mode: "public", value: "true"}
}

func (r RoutingExportMode) ApplyToMeta(obj metav1.Object) {
	addAnnotation(r, obj)
}

func (r RoutingExportMode) Key() string {
	return RoutingExportModePrefix + r.mode
}

func (r RoutingExportMode) Value() string {
	return r.value
}

// RoutingAddressesPublic exposes the public addresses set by Platform's routing capability.
// It is set by the Platform's Routing controller back to the component's Custom Resource.
// Values are delimited by ";".
type RoutingAddressesPublic string

func (r RoutingAddressesPublic) ApplyToMeta(obj metav1.Object) {
	addAnnotation(r, obj)
}

func (r RoutingAddressesPublic) Key() string {
	return "routing.opendatahub.io/public-addresses"
}

func (r RoutingAddressesPublic) Value() string {
	return string(r)
}

// RoutingAddressesExternal exposes the external addresses set by Platform's routing capability.
// It is set by the Platform's Routing controller back to the component's Custom Resource.
// Values are delimited by ";".
type RoutingAddressesExternal string

func (r RoutingAddressesExternal) ApplyToMeta(obj metav1.Object) {
	addAnnotation(r, obj)
}

func (r RoutingAddressesExternal) Key() string {
	return "routing.opendatahub.io/external-addresses"
}

func (r RoutingAddressesExternal) Value() string {
	return string(r)
}

func addAnnotation(annotation Annotation, obj metav1.Object) {
	existingAnnotations := obj.GetAnnotations()
	if existingAnnotations == nil {
		existingAnnotations = make(map[string]string)
	}

	existingAnnotations[annotation.Key()] = annotation.Value()
	obj.SetAnnotations(existingAnnotations)
}
