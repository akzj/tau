package core

// Version is set at build time via ldflags: -X github.com/akzj/tau/core.Version=v1.0.0
var Version = "dev"

// BuildTime is set at build time via ldflags.
var BuildTime = "unknown"

// CommitSHA is set at build time via ldflags.
var CommitSHA = "unknown"
