package metadata

var Finalizers = struct { //nolint:gochecknoglobals //reason: anonymous struct is used for grouping finalizers together instead of consts
	Routing string
}{
	Routing: "routing.finalizer.opendatahub.io",
}
