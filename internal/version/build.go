package version

import "runtime/debug"

// Version is the release tag this binary was built from. Release builds stamp
// it via:
//
//	-ldflags "-X github.com/0x3639/go-syrius/internal/version.Version=v0.3.10"
//
// Anything unstamped (local wails dev/build, go test) reports "dev" so a
// non-release binary can never impersonate a release.
var Version = "dev"

// CommitOverride is the full commit hash stamped via -ldflags -X by release
// builds. It exists because wails build suppresses the Go toolchain's
// automatic VCS stamping, so releases pass the commit explicitly; when set it
// takes precedence over build info.
var CommitOverride = ""

// Commit returns the short VCS revision this binary was built from: the
// -ldflags CommitOverride when stamped, else the toolchain's vcs.revision
// (with a "-dirty" suffix for a modified tree), else "unknown" (test
// binaries, non-git checkouts, unstamped wails builds).
func Commit() string {
	if CommitOverride != "" {
		return shortHash(CommitOverride)
	}
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return "unknown"
	}
	return commitFromBuildInfo(info)
}

func shortHash(rev string) string {
	if len(rev) > 7 {
		return rev[:7]
	}
	return rev
}

func commitFromBuildInfo(info *debug.BuildInfo) string {
	var revision string
	var dirty bool
	for _, s := range info.Settings {
		switch s.Key {
		case "vcs.revision":
			revision = s.Value
		case "vcs.modified":
			dirty = s.Value == "true"
		}
	}
	if revision == "" {
		return "unknown"
	}
	revision = shortHash(revision)
	if dirty {
		revision += "-dirty"
	}
	return revision
}
