package metadata

var Annotations = struct { //nolint:gochecknoglobals //reason: anonymous struct is used for grouping annotations together instead of consts
	AuthEnabled              string
	AuthorizationGroup       string
	RoutingExportMode        string
	RoutingAddressesPublic   string
	RoutingAddressesExternal string
}{
	AuthEnabled:              "security.opendatahub.io/enable-auth",
	AuthorizationGroup:       "security.opendatahub.io/authorization-group",
	RoutingExportMode:        "routing.opendatahub.io/export-mode",
	RoutingAddressesPublic:   "routing.opendatahub.io/addresses-public",
	RoutingAddressesExternal: "routing.opendatahub.io/addresses-external",
}
