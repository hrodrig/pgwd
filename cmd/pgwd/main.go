// Package main is the pgwd binary entrypoint. Logic lives in internal/cli and internal/run.
package main

import (
	"github.com/hrodrig/pgwd/internal/cli"
)

// Set at build time via -ldflags (see Makefile).
var (
	Version   string = "dev"
	Commit    string = ""
	BuildDate string = ""
	Branch    string = ""
)

func main() {
	cli.SetBuildInfo(Version, Commit, BuildDate, Branch)
	cli.Run()
}
