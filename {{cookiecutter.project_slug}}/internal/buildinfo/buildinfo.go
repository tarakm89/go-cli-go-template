// Package buildinfo carries the identity of the binary. The values are set at
// link time by the Makefile and by the release workflow; when the binary was
// built by a plain `go build` they are recovered from the embedded VCS stamps
// instead, so `version` is never a lie.
package buildinfo

import (
	"fmt"
	"runtime"
	"runtime/debug"
)

// Injected via -ldflags at build time. Do not rename without updating the
// LDFLAGS in the Makefile.
var (
	version = "dev"
	commit  = ""
	date    = ""
)

// Info describes the running binary.
type Info struct {
	Name      string `json:"name"`
	Version   string `json:"version"`
	Commit    string `json:"commit"`
	Date      string `json:"date"`
	GoVersion string `json:"go_version"`
	Platform  string `json:"platform"`
}

// Name is the binary's canonical name, used for the CLI, the OTel service name
// and the User-Agent sent to external systems.
const Name = "{{ cookiecutter.binary_name }}"

// Get assembles the build information, filling any gaps from the module's
// embedded VCS stamps.
func Get() Info {
	info := Info{
		Name:      Name,
		Version:   version,
		Commit:    commit,
		Date:      date,
		GoVersion: runtime.Version(),
		Platform:  runtime.GOOS + "/" + runtime.GOARCH,
	}

	build, ok := debug.ReadBuildInfo()
	if !ok {
		return info
	}
	for _, setting := range build.Settings {
		switch setting.Key {
		case "vcs.revision":
			if info.Commit == "" {
				info.Commit = setting.Value
			}
		case "vcs.time":
			if info.Date == "" {
				info.Date = setting.Value
			}
		case "vcs.modified":
			if setting.Value == "true" {
				info.Version += "+dirty"
			}
		}
	}

	return info
}

// String renders a single line suitable for `--version`.
func (i Info) String() string {
	commit := i.Commit
	if len(commit) > 12 {
		commit = commit[:12]
	}
	return fmt.Sprintf("%s %s (commit %s, built %s, %s, %s)",
		i.Name, i.Version, orUnknown(commit), orUnknown(i.Date), i.GoVersion, i.Platform)
}

func orUnknown(value string) string {
	if value == "" {
		return "unknown"
	}
	return value
}
