package metadata

import "github.com/opendatahub-io/odh-platform/version"

const (
	LabelAppPartOf       = "app.kubernetes.io/part-of"
	LabelAppComponent    = "app.kubernetes.io/component"
	LabelAppName         = "app.kubernetes.io/name"
	LabelAppVersion      = "app.kubernetes.io/version"
	LabelAppManagedBy    = "app.kubernetes.io/managed-by"
	LabelOwnerName       = "platform.opendatahub.io/owner-name"
	LabelRoutingExported = "routing.opendatahub.io/exported"
)

func ApplyStandard(source map[string]string) map[string]string {
	target := map[string]string{}

	target[LabelAppPartOf] = source[LabelAppName]
	target[LabelAppComponent] = source[LabelAppComponent]

	target[LabelAppVersion] = version.Version
	target[LabelAppManagedBy] = "odh-platform"

	return target
}
