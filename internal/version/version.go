package version

// Set via ldflags at build time:
//
//	go build -ldflags "-X github.com/BorisWilhelms/parentald/internal/version.Version=abc123"
var Version = "dev"
