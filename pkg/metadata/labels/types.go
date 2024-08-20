package labels

import (
	"github.com/opendatahub-io/odh-platform/pkg/metadata"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type Label interface {
	metadata.Option
	metadata.KeyValue
}

// K8s Recommended Labels
// see: https://kubernetes.io/docs/concepts/overview/working-with-objects/common-labels/#labels

type AppPartOf string

func (a AppPartOf) ApplyToMeta(obj metav1.Object) {
	addLabel(a, obj)
}

func (a AppPartOf) Key() string {
	return "app.kubernetes.io/part-of"
}

func (a AppPartOf) Value() string {
	return string(a)
}

type AppComponent string

func (a AppComponent) ApplyToMeta(obj metav1.Object) {
	addLabel(a, obj)
}

func (a AppComponent) Key() string {
	return "app.kubernetes.io/component"
}

func (a AppComponent) Value() string {
	return string(a)
}

type AppName string

func (a AppName) ApplyToMeta(obj metav1.Object) {
	addLabel(a, obj)
}

func (a AppName) Key() string {
	return "app.kubernetes.io/name"
}

func (a AppName) Value() string {
	return string(a)
}

type AppVersion string

func (a AppVersion) ApplyToMeta(obj metav1.Object) {
	addLabel(a, obj)
}

func (a AppVersion) Key() string {
	return "app.kubernetes.io/version"
}

func (a AppVersion) Value() string {
	return string(a)
}

type AppManagedBy string

func (a AppManagedBy) ApplyToMeta(obj metav1.Object) {
	addLabel(a, obj)
}

func (a AppManagedBy) Key() string {
	return "app.kubernetes.io/managed-by"
}

func (a AppManagedBy) Value() string {
	return string(a)
}

// Platform Specific Labels

// OwnerName is the name of the owner of the resource.
type OwnerName string

func (o OwnerName) ApplyToMeta(obj metav1.Object) {
	addLabel(o, obj)
}

func (o OwnerName) Key() string {
	return "platform.opendatahub.io/owner-name"
}

func (o OwnerName) Value() string {
	return string(o)
}

// OwnerKind is the kind of the owner of the resource.
type OwnerKind string

func (o OwnerKind) ApplyToMeta(obj metav1.Object) {
	addLabel(o, obj)
}

func (o OwnerKind) Key() string {
	return "platform.opendatahub.io/owner-kind"
}

func (o OwnerKind) Value() string {
	return string(o)
}

// OwnerUID is the UID of the owner of the resource. It is internally set by the platform
// to enable accurate garbage collection of the resources cross-namespace.
type OwnerUID string

func (o OwnerUID) ApplyToMeta(obj metav1.Object) {
	addLabel(o, obj)
}

func (o OwnerUID) Key() string {
	return "platform.opendatahub.io/owner-uid"
}

func (o OwnerUID) Value() string {
	return string(o)
}

// RoutingExported is a Label to mark resources that are exported by the routing capability.
// It is intended to be set by enrolled component to mark resources that should be used to
// configure routing capability by the Platform. This can be a Kubernetes Service or Istio
// VirtualService from which settings like hosts and ports are extracted.
type RoutingExported string

func (r RoutingExported) ApplyToMeta(obj metav1.Object) {
	addLabel(r, obj)
}

func (r RoutingExported) Key() string {
	return "routing.opendatahub.io/exported"
}

func (r RoutingExported) Value() string {
	return string(r)
}

func addLabel(label Label, obj metav1.Object) {
	existingLabels := obj.GetLabels()
	if existingLabels == nil {
		existingLabels = make(map[string]string)
	}

	existingLabels[label.Key()] = label.Value()
	obj.SetLabels(existingLabels)
}