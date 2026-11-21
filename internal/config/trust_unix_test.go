//go:build unix

package config

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"
)

type fakeInfo struct {
	mode fs.FileMode
	uid  uint32
	id   string // file identity, for the same-file check
}

func (f fakeInfo) Name() string       { return "x" }
func (f fakeInfo) Size() int64        { return 0 }
func (f fakeInfo) Mode() fs.FileMode  { return f.mode }
func (f fakeInfo) ModTime() time.Time { return time.Time{} }
func (f fakeInfo) IsDir() bool        { return f.mode.IsDir() }
func (f fakeInfo) Sys() any           { return &syscall.Stat_t{Uid: f.uid} }

var (
	rootDir  = fakeInfo{mode: fs.ModeDir | 0o755}
	rootFile = fakeInfo{mode: 0o600, id: "config"}
)

func sameFakeFile(a, b fs.FileInfo) bool { return a.(fakeInfo).id == b.(fakeInfo).id }

// trustTree is a fake filesystem: path → FileInfo, plus symlink resolution.
type trustTree struct {
	t     *testing.T
	infos map[string]fakeInfo
	links map[string]string
}

func (tr trustTree) stat(p string) (fs.FileInfo, error) {
	fi, ok := tr.infos[p]
	if !ok {
		tr.t.Fatalf("unexpected stat of %s", p)
	}
	return fi, nil
}

func (tr trustTree) resolve(p string) (string, error) {
	if r, ok := tr.links[p]; ok {
		return r, nil
	}
	return p, nil
}

func (tr trustTree) check(name string) error {
	return checkTrustedWith(name, tr.infos[name], tr.stat, tr.resolve, sameFakeFile)
}

func TestCheckTrusted(t *testing.T) {
	const file = "/etc/gluper/config.yaml"
	base := func() map[string]fakeInfo {
		return map[string]fakeInfo{file: rootFile, "/etc/gluper": rootDir, "/etc": rootDir, "/": rootDir}
	}
	tests := []struct {
		name   string
		change func(m map[string]fakeInfo)
		want   string // "" = trusted
	}{
		{"root owned", func(map[string]fakeInfo) {}, ""},
		{"file owned by user", func(m map[string]fakeInfo) { m[file] = fakeInfo{mode: 0o600, uid: 1000, id: "config"} }, "owned by uid 1000"},
		{"file group writable", func(m map[string]fakeInfo) { m[file] = fakeInfo{mode: 0o620, id: "config"} }, "writable by group or others"},
		{"parent world writable", func(m map[string]fakeInfo) { m["/etc/gluper"] = fakeInfo{mode: fs.ModeDir | 0o777} }, "/etc/gluper is writable"},
		{"parent owned by user", func(m map[string]fakeInfo) { m["/etc/gluper"] = fakeInfo{mode: fs.ModeDir | 0o755, uid: 1000} }, "/etc/gluper is owned by uid 1000"},
		{"grandparent writable", func(m map[string]fakeInfo) { m["/etc"] = fakeInfo{mode: fs.ModeDir | 0o775} }, "/etc is writable"},
		{"root dir writable", func(m map[string]fakeInfo) { m["/"] = fakeInfo{mode: fs.ModeDir | 0o777} }, "/ is writable"},
		{"sticky dir not exempt", func(m map[string]fakeInfo) { m["/etc"] = fakeInfo{mode: fs.ModeDir | fs.ModeSticky | 0o777} }, "/etc is writable"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			infos := base()
			tt.change(infos)
			err := trustTree{t: t, infos: infos}.check(file)
			switch {
			case tt.want == "" && err != nil:
				t.Errorf("want trusted, got %v", err)
			case tt.want != "" && (err == nil || !strings.Contains(err.Error(), tt.want)):
				t.Errorf("want error containing %q, got %v", tt.want, err)
			}
		})
	}
}

func TestCheckTrustedSwappedFile(t *testing.T) {
	const file = "/etc/gluper/config.yaml"
	tr := trustTree{t: t, infos: map[string]fakeInfo{
		file: rootFile, "/etc/gluper": rootDir, "/etc": rootDir, "/": rootDir,
	}}
	// The descriptor gluper opened is a different file from the one now at
	// the path.
	opened := fakeInfo{mode: 0o600, id: "other"}
	err := checkTrustedWith(file, opened, tr.stat, tr.resolve, sameFakeFile)
	if err == nil || !strings.Contains(err.Error(), "changed while it was being opened") {
		t.Errorf("got %v", err)
	}
}

func TestCheckTrustedFollowsSymlinks(t *testing.T) {
	const link, target = "/etc/gluper/config.yaml", "/srv/managed/gluper.yaml"
	tr := trustTree{
		t: t,
		infos: map[string]fakeInfo{
			link: rootFile, target: rootFile,
			"/etc/gluper": rootDir, "/etc": rootDir, "/": rootDir, "/srv": rootDir,
			"/srv/managed": {mode: fs.ModeDir | 0o777}, // the link target's directory is writable
		},
		links: map[string]string{link: target},
	}
	err := tr.check(link)
	if err == nil || !strings.Contains(err.Error(), "/srv/managed is writable") {
		t.Errorf("want the target directory refused, got %v", err)
	}
}

// TestCheckTrustedReal runs the real check against a temporary directory. As
// a normal user it must be refused; when the tests run as root (for example
// in a container) a 0700 directory under a root-owned tree must pass.
func TestCheckTrustedReal(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(p, []byte("target: latest\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	f, err := os.Open(p)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	fi, err := f.Stat()
	if err != nil {
		t.Fatal(err)
	}
	err = checkTrusted(p, fi)
	if os.Geteuid() != 0 && err == nil {
		t.Error("a file owned by a normal user must not be trusted")
	}
	if err != nil {
		t.Logf("refused as expected or by an ancestor: %v", err)
	}
}
