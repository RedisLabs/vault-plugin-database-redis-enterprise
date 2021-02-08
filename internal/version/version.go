package version

var (
	// Version is the Git tag the plugin was built from - it is injected at build time for releases
	Version = "dev"
	// GitCommit is the commit the plugin was built from - it is injected at build time for releases
	GitCommit = "dev"
)
