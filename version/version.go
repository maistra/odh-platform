package version

//nolint:gochecknoglobals //reason these are used for binary version output only
var (
	// Version hold a semantic version of the running binary.
	Version = "v0.0.0"
	// Commit holds the commit hash against which the binary build was ran.
	Commit string
	// BuildTime holds timestamp when the binary build was ran.
	BuildTime string
)
