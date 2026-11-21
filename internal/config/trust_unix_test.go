//go:build unix

package config

import (
	"io/fs"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"
)

type fakeInfo struct {
	mode fs.FileMode
	uid  uint32
}

func (f fakeInfo) Name() string       { return "x" }
func (f fakeInfo) Size() int64        { return 0 }
func (f fakeInfo) Mode() fs.FileMode  { return f.mode }
func (f fakeInfo) ModTime() time.Time { return time.Time{} }
func (f fakeInfo) IsDir() bool        { return f.mode.IsDir() }
func (f fakeInfo) Sys() any           { return &syscall.Stat_t{Uid: f.uid} }

func TestCheckTrusted(t *testing.T) {
	file := "/etc/gluper/config.yaml"
	dir := filepath.Dir(file)
	noLinks := func(p string) (string, error) { return p, nil }
	tests := []struct {
		name  string
		infos map[string]fakeInfo
		want  string // "" = trusted
	}{
		{"root owned", map[string]fakeInfo{file: {0o600, 0}, dir: {fs.ModeDir | 0o755, 0}}, ""},
		{"file owned by user", map[string]fakeInfo{file: {0o600, 1000}, dir: {fs.ModeDir | 0o755, 0}}, "owned by uid 1000"},
		{"file group writable", map[string]fakeInfo{file: {0o620, 0}, dir: {fs.ModeDir | 0o755, 0}}, "writable by group or others"},
		{"dir world writable", map[string]fakeInfo{file: {0o600, 0}, dir: {fs.ModeDir | 0o777, 0}}, "/etc/gluper is writable"},
		{"dir owned by user", map[string]fakeInfo{file: {0o600, 0}, dir: {fs.ModeDir | 0o755, 1000}}, "/etc/gluper is owned by uid 1000"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stat := func(p string) (fs.FileInfo, error) {
				fi, ok := tt.infos[p]
				if !ok {
					t.Fatalf("unexpected stat of %s", p)
				}
				return fi, nil
			}
			err := checkTrustedWith(file, stat, noLinks)
			switch {
			case tt.want == "" && err != nil:
				t.Errorf("want trusted, got %v", err)
			case tt.want != "" && (err == nil || !strings.Contains(err.Error(), tt.want)):
				t.Errorf("want error containing %q, got %v", tt.want, err)
			}
		})
	}
}

func TestCheckTrustedFollowsSymlinks(t *testing.T) {
	link, target := "/etc/gluper/config.yaml", "/srv/managed/gluper.yaml"
	infos := map[string]fakeInfo{
		target:         {0o644, 0},
		"/srv/managed": {fs.ModeDir | 0o777, 0}, // the link's target directory is writable
		"/etc/gluper":  {fs.ModeDir | 0o755, 0},
	}
	stat := func(p string) (fs.FileInfo, error) { return infos[p], nil }
	resolve := func(string) (string, error) { return target, nil }
	err := checkTrustedWith(link, stat, resolve)
	if err == nil || !strings.Contains(err.Error(), "/srv/managed") {
		t.Errorf("want the target directory refused, got %v", err)
	}
}
