package metadata

import "github.com/opendatahub-io/odh-platform/version"

var Labels = struct { //nolint:gochecknoglobals //reason: anonymous struct is used for grouping labels together instead of consts
	AppPartOf    string
	AppComponent string
	AppName      string
	AppVersion   string
	AppManagedBy string
	// OwnerName is the name of the owner of the resource.
	OwnerName string
	// OwnerKind is the kind of the owner of the resource.
	OwnerKind string
	// OwnerUID is the UID of the owner of the resource. It is internally set by the platform
	// to enable accurate garbage collection of the resources cross-namespace.
	OwnerUID        string
	RoutingExported string
	ExportType      string
}{
	AppPartOf:       "app.kubernetes.io/part-of",
	AppComponent:    "app.kubernetes.io/component",
	AppName:         "app.kubernetes.io/name",
	AppVersion:      "app.kubernetes.io/version",
	AppManagedBy:    "app.kubernetes.io/managed-by",
	OwnerName:       "platform.opendatahub.io/owner-name",
	OwnerKind:       "platform.opendatahub.io/owner-kind",
	OwnerUID:        "platform.opendatahub.io/owner-uid",
	RoutingExported: "routing.opendatahub.io/exported",
	ExportType:      "routing.opendatahub.io/type",
}

func ApplyStandard(source map[string]string) map[string]string {
	target := map[string]string{}

	target[Labels.AppPartOf] = source[Labels.AppName]
	target[Labels.AppComponent] = source[Labels.AppComponent]

	target[Labels.AppVersion] = version.Version
	target[Labels.AppManagedBy] = "odh-platform"

	return target
}
