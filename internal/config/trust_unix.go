//go:build unix

package config

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"syscall"
)

// maxSymlinks bounds symlink resolution, like the kernel's ELOOP limit.
const maxSymlinks = 40

// checkTrusted refuses a config file that a user other than root could have
// written or swapped. gluper runs hooks as root, so a writable config would
// let another user run commands as root.
//
// The path is resolved one component at a time, following each symlink
// manually. Every directory gluper looks a name up in must be owned by root
// and not writable by group or others: the directories along the given
// path, the directories holding each symlink, and the directories of every
// link target. Like OpenSSH, sticky directories such as /tmp get no
// exception. No other user can change a directory that passes this check,
// so the resolved path can't be redirected after the check.
//
// opened is the FileInfo of the descriptor gluper actually reads. It must be
// trusted too, and it must be the file the path resolves to.
func checkTrusted(name string, opened fs.FileInfo) error {
	return checkTrustedWith(name, opened, trustFS{lstat: os.Lstat, readlink: os.Readlink, same: os.SameFile})
}

// trustFS holds the filesystem calls checkTrusted makes, so tests can fake
// them.
type trustFS struct {
	lstat    func(string) (fs.FileInfo, error)
	readlink func(string) (string, error)
	same     func(a, b fs.FileInfo) bool
}

func checkTrustedWith(name string, opened fs.FileInfo, tfs trustFS) error {
	if err := checkOwner(name, opened, true); err != nil {
		return err
	}
	resolved, final, err := resolveTrusted(name, tfs)
	if err != nil {
		return err
	}
	if !tfs.same(final, opened) {
		return fmt.Errorf("the config file changed while it was being opened (%s)", resolved)
	}
	return nil
}

// resolveTrusted resolves name to a path without symlinks, checking every
// directory it looks a name up in. It returns the resolved path and its
// FileInfo.
func resolveTrusted(name string, tfs trustFS) (string, fs.FileInfo, error) {
	abs, err := filepath.Abs(name)
	if err != nil {
		return "", nil, err
	}
	checked := map[string]bool{}
	links := 0
	enter := func(dir string, fi fs.FileInfo) error {
		if checked[dir] {
			return nil
		}
		if !fi.IsDir() {
			return fmt.Errorf("%s is not a directory", dir)
		}
		// Before any symlink, the directory is on the path the admin chose,
		// so fixing its owner is the right advice. After one, it's somewhere
		// a link leads, and the link should not go through it.
		if err := checkOwner(dir, fi, links == 0); err != nil {
			return err
		}
		checked[dir] = true
		return nil
	}

	rootInfo, err := tfs.lstat("/")
	if err != nil {
		return "", nil, err
	}
	if err := enter("/", rootInfo); err != nil {
		return "", nil, err
	}

	cur, curInfo := "/", rootInfo
	pending := components(abs)
	for len(pending) > 0 {
		c := pending[0]
		pending = pending[1:]
		switch c {
		case "", ".":
			continue
		case "..":
			// cur has no symlinks left, so its parent is the real parent.
			cur = filepath.Dir(cur)
			if curInfo, err = tfs.lstat(cur); err != nil {
				return "", nil, err
			}
			continue
		}
		// Looking c up in cur: cur was checked when it was entered.
		next := filepath.Join(cur, c)
		fi, err := tfs.lstat(next)
		if err != nil {
			return "", nil, err
		}
		if fi.Mode()&fs.ModeSymlink != 0 {
			if links++; links > maxSymlinks {
				return "", nil, fmt.Errorf("%s: too many levels of symbolic links", abs)
			}
			target, err := tfs.readlink(next)
			if err != nil {
				return "", nil, err
			}
			if filepath.IsAbs(target) {
				cur = "/"
				if curInfo, err = tfs.lstat(cur); err != nil {
					return "", nil, err
				}
			}
			pending = append(components(target), pending...)
			continue
		}
		cur, curInfo = next, fi
		if len(pending) > 0 {
			if err := enter(cur, fi); err != nil {
				return "", nil, err
			}
		}
	}
	if curInfo.IsDir() {
		return "", nil, errors.New("the config file is a directory")
	}
	return cur, curInfo, nil
}

func components(p string) []string {
	return strings.Split(filepath.ToSlash(p), "/")
}

// checkOwner requires p to be owned by root and not writable by group or
// others. fixHint adds the chown/chmod command that fixes it.
func checkOwner(p string, fi fs.FileInfo, fixHint bool) error {
	st, ok := fi.Sys().(*syscall.Stat_t)
	if !ok {
		return fmt.Errorf("%s: no ownership information", p)
	}
	var problem, fix string
	switch {
	case st.Uid != 0:
		problem, fix = fmt.Sprintf("%s is owned by uid %d, not root", p, st.Uid), "chown root "+p
	case fi.Mode().Perm()&0o022 != 0:
		problem, fix = fmt.Sprintf("%s is writable by group or others (mode %04o)", p, fi.Mode().Perm()), "chmod go-w "+p
	default:
		return nil
	}
	if fixHint {
		return fmt.Errorf("%s; run: %s", problem, fix)
	}
	return fmt.Errorf("%s, and a symlink to the config file leads through it; keep the file and every link to it in root-owned directories", problem)
}
