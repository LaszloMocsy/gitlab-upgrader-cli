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
	mode   fs.FileMode
	uid    uint32
	id     string // file identity, for the same-file check
	target string // symlink target
}

func (f fakeInfo) Name() string       { return "x" }
func (f fakeInfo) Size() int64        { return 0 }
func (f fakeInfo) Mode() fs.FileMode  { return f.mode }
func (f fakeInfo) ModTime() time.Time { return time.Time{} }
func (f fakeInfo) IsDir() bool        { return f.mode.IsDir() }
func (f fakeInfo) Sys() any           { return &syscall.Stat_t{Uid: f.uid} }

var (
	rootDir = fakeInfo{mode: fs.ModeDir | 0o755}
	userDir = fakeInfo{mode: fs.ModeDir | 0o755, uid: 1000}
	openDir = fakeInfo{mode: fs.ModeDir | 0o777}
)

func rootFile(id string) fakeInfo { return fakeInfo{mode: 0o600, id: id} }
func link(target string) fakeInfo {
	return fakeInfo{mode: fs.ModeSymlink | 0o777, uid: 1000, target: target}
}

// tree is a fake filesystem keyed by absolute path.
type tree map[string]fakeInfo

func (tr tree) fs(t *testing.T) trustFS {
	return trustFS{
		lstat: func(p string) (fs.FileInfo, error) {
			fi, ok := tr[p]
			if !ok {
				return nil, &fs.PathError{Op: "lstat", Path: p, Err: fs.ErrNotExist}
			}
			return fi, nil
		},
		readlink: func(p string) (string, error) {
			if tr[p].target == "" {
				t.Fatalf("readlink of non-link %s", p)
			}
			return tr[p].target, nil
		},
		same: func(a, b fs.FileInfo) bool { return a.(fakeInfo).id == b.(fakeInfo).id },
	}
}

// base is a trusted /etc/gluper/config.yaml.
func base() tree {
	return tree{
		"/":                       rootDir,
		"/etc":                    rootDir,
		"/etc/gluper":             rootDir,
		"/etc/gluper/config.yaml": rootFile("config"),
		"/srv":                    rootDir,
		"/srv/real":               rootDir,
		"/srv/real/config.yaml":   rootFile("real"),
		"/home":                   rootDir,
		"/home/user":              userDir,
		"/tmp":                    {mode: fs.ModeDir | fs.ModeSticky | 0o777},
	}
}

func TestCheckTrusted(t *testing.T) {
	const file = "/etc/gluper/config.yaml"
	tests := []struct {
		name   string
		change func(tr tree)
		opened fakeInfo
		want   string // "" = trusted
	}{
		{"root owned", func(tree) {}, rootFile("config"), ""},
		{"opened file owned by user", func(tree) {}, fakeInfo{mode: 0o600, uid: 1000, id: "config"}, "owned by uid 1000"},
		{"opened file group writable", func(tree) {}, fakeInfo{mode: 0o620, id: "config"}, "writable by group or others"},
		{"parent world writable", func(tr tree) { tr["/etc/gluper"] = openDir }, rootFile("config"), "/etc/gluper is writable"},
		{"parent owned by user", func(tr tree) { tr["/etc/gluper"] = userDir }, rootFile("config"), "/etc/gluper is owned by uid 1000, not root; run: chown root /etc/gluper"},
		{"grandparent writable", func(tr tree) { tr["/etc"] = fakeInfo{mode: fs.ModeDir | 0o775} }, rootFile("config"), "/etc is writable"},
		{"root dir writable", func(tr tree) { tr["/"] = openDir }, rootFile("config"), "/ is writable"},
		{"swapped after open", func(tree) {}, rootFile("other"), "changed while it was being opened"},
		{"missing", func(tr tree) { delete(tr, file) }, rootFile("config"), "not exist"},
		{"path through a file", func(tr tree) { tr["/etc/gluper"] = rootFile("x") }, rootFile("config"), "not a directory"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tr := base()
			tt.change(tr)
			wantTrust(t, checkTrustedWith(file, tt.opened, tr.fs(t)), tt.want)
		})
	}
}

func TestCheckTrustedSymlinks(t *testing.T) {
	const file = "/etc/gluper/config.yaml"
	tests := []struct {
		name   string
		change func(tr tree)
		opened string
		want   string
	}{
		{"absolute link into trusted dir", func(tr tree) { tr[file] = link("/srv/real/config.yaml") }, "real", ""},
		{"relative link with ..", func(tr tree) { tr[file] = link("../../srv/real/config.yaml") }, "real", ""},
		{"linked directory", func(tr tree) {
			tr["/etc/gluper"] = link("/srv/real")
			tr["/srv/real/config.yaml"] = rootFile("real")
		}, "real", ""},
		{"target dir writable", func(tr tree) {
			tr[file] = link("/srv/real/config.yaml")
			tr["/srv/real"] = openDir
		}, "real", "/srv/real is writable by group or others (mode 0777), and a symlink"},
		{"link chain through a user-writable dir", func(tr tree) {
			// /etc/gluper/config.yaml -> /home/user/cfg -> /srv/real/config.yaml.
			// The user can repoint /home/user/cfg, so the chain is refused even
			// though both ends are trusted.
			tr[file] = link("/home/user/cfg")
			tr["/home/user/cfg"] = link("/srv/real/config.yaml")
		}, "real", "/home/user is owned by uid 1000, not root, and a symlink"},
		{"link via sticky /tmp", func(tr tree) {
			tr[file] = link("/tmp/cfg")
			tr["/tmp/cfg"] = link("/srv/real/config.yaml")
		}, "real", "/tmp is writable"},
		{"link loop", func(tr tree) {
			tr[file] = link("/etc/gluper/loop")
			tr["/etc/gluper/loop"] = link("/etc/gluper/config.yaml")
		}, "real", "too many levels"},
		{"link to a directory", func(tr tree) { tr[file] = link("/srv/real") }, "real", "is a directory"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tr := base()
			tt.change(tr)
			wantTrust(t, checkTrustedWith(file, rootFile(tt.opened), tr.fs(t)), tt.want)
		})
	}
}

func wantTrust(t *testing.T, err error, want string) {
	t.Helper()
	switch {
	case want == "" && err != nil:
		t.Errorf("want trusted, got %v", err)
	case want != "" && (err == nil || !strings.Contains(err.Error(), want)):
		t.Errorf("want error containing %q, got %v", want, err)
	}
}

// TestCheckTrustedReal runs the real check against a temporary file. The
// temporary directory is never trusted: it belongs to a normal user, or (as
// root) sits under a world-writable /tmp.
func TestCheckTrustedReal(t *testing.T) {
	p := filepath.Join(t.TempDir(), "config.yaml")
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
	if err := checkTrusted(p, fi); err == nil {
		t.Error("a file in a temporary directory must not be trusted")
	} else {
		t.Logf("refused: %v", err)
	}
}
