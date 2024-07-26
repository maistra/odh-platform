package controllers

const (
	AnnotationOpendatahub = "opendatahub.io/dashboard" // TODO: Just temp to narrow project

	// Auth.
	AnnotationAuthEnabled        = "security.opendatahub.io/enable-auth"
	AnnotationAuthorizationGroup = "security.opendatahub.io/authorization-group"

	// Routing.
	AnnotationRoutingExportMode         = "routing.opendatahub.io/export-mode"
	AnnotationRoutingAdddressesPublic   = "routing.opendatahub.io/addresses-public"
	AnnotationRoutingAdddressesExternal = "routing.opendatahub.io/addresses-external"
)
