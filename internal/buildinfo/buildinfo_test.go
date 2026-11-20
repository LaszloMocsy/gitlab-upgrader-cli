package buildinfo

import (
	"runtime"
	"runtime/debug"
	"testing"
)

func TestFillFromBuildInfo(t *testing.T) {
	bi := &debug.BuildInfo{
		Main: debug.Module{Version: "v1.2.3"},
		Settings: []debug.BuildSetting{
			{Key: "vcs.revision", Value: "abc123"},
			{Key: "vcs.time", Value: "2026-11-20T00:00:00Z"},
			{Key: "vcs.modified", Value: "true"},
		},
	}

	t.Run("go install build uses module and VCS data", func(t *testing.T) {
		info := Info{Version: "dev"}
		fillFromBuildInfo(&info, bi)
		want := Info{Version: "v1.2.3", Commit: "abc123", Date: "2026-11-20T00:00:00Z", Modified: true}
		if info != want {
			t.Errorf("got %+v, want %+v", info, want)
		}
	})

	t.Run("linker values win", func(t *testing.T) {
		info := Info{Version: "1.0.0", Commit: "fromldflags", Date: "2026-01-01T00:00:00Z"}
		fillFromBuildInfo(&info, bi)
		if info.Version != "1.0.0" || info.Commit != "fromldflags" || info.Date != "2026-01-01T00:00:00Z" {
			t.Errorf("linker values were overwritten: %+v", info)
		}
	})

	t.Run("devel module version is ignored", func(t *testing.T) {
		info := Info{Version: "dev"}
		fillFromBuildInfo(&info, &debug.BuildInfo{Main: debug.Module{Version: "(devel)"}})
		if info.Version != "dev" {
			t.Errorf("Version = %q, want dev", info.Version)
		}
	})
}

func TestGetReportsPlatform(t *testing.T) {
	info := Get()
	if info.OS != runtime.GOOS || info.Arch != runtime.GOARCH || info.GoVersion != runtime.Version() {
		t.Errorf("unexpected platform fields: %+v", info)
	}
	if info.Version == "" {
		t.Error("Version is empty")
	}
}
