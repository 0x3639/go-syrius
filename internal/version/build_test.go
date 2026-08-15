package version

import (
	"runtime/debug"
	"testing"
)

func TestVersionDefaultsToDev(t *testing.T) {
	// Release builds override this via -ldflags -X; anything else must be
	// honest about not being a release.
	if Version != "dev" {
		t.Fatalf("Version = %q, want \"dev\" (unstamped build)", Version)
	}
}

func TestCommitFromBuildInfo(t *testing.T) {
	cases := []struct {
		name     string
		settings []debug.BuildSetting
		want     string
	}{
		{
			name: "clean revision truncated to short hash",
			settings: []debug.BuildSetting{
				{Key: "vcs.revision", Value: "224cc361234567890abcdef01234567890abcdef"},
				{Key: "vcs.modified", Value: "false"},
			},
			want: "224cc36",
		},
		{
			name: "dirty tree flagged",
			settings: []debug.BuildSetting{
				{Key: "vcs.revision", Value: "224cc361234567890abcdef01234567890abcdef"},
				{Key: "vcs.modified", Value: "true"},
			},
			want: "224cc36-dirty",
		},
		{
			name:     "no vcs stamping",
			settings: nil,
			want:     "unknown",
		},
		{
			name: "short revision passed through",
			settings: []debug.BuildSetting{
				{Key: "vcs.revision", Value: "abc"},
			},
			want: "abc",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			info := &debug.BuildInfo{Settings: tc.settings}
			if got := commitFromBuildInfo(info); got != tc.want {
				t.Fatalf("commitFromBuildInfo = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestCommitNeverEmpty(t *testing.T) {
	// Test binaries are built without VCS stamping, so this exercises the
	// fallback path in-process: whatever the environment, Commit() must return
	// something displayable.
	if Commit() == "" {
		t.Fatal("Commit() returned empty string")
	}
}

func TestCommitOverrideWins(t *testing.T) {
	// wails build disables the toolchain's automatic VCS stamping, so releases
	// stamp the commit via -ldflags; the override must beat build info and get
	// the same short-hash treatment.
	old := CommitOverride
	defer func() { CommitOverride = old }()
	CommitOverride = "fa78b2946ca3316c65c1f245f7d1bb4c0e39df6a"
	if got := Commit(); got != "fa78b29" {
		t.Fatalf("Commit() with override = %q, want \"fa78b29\"", got)
	}
}
