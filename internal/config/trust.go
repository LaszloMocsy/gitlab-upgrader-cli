package config

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
)

// checkTrusted refuses a config file that a user other than root could have
// written or swapped. gluper runs hooks as root, so a writable config would
// let another user run commands as root.
//
// opened is the FileInfo of the descriptor gluper actually reads, so the file
// check can't race with a rename. Every directory from the file up to the
// root, both along the path as given and after resolving symlinks, must be
// owned by root and not writable by group or others. Like OpenSSH, there is
// no exception for sticky directories such as /tmp. With all ancestors locked
// down, no other user can swap the file between the check and the read.
func checkTrusted(name string, opened fs.FileInfo) error {
	return checkTrustedWith(name, opened, os.Stat, filepath.EvalSymlinks, os.SameFile)
}

func checkTrustedWith(name string, opened fs.FileInfo, stat func(string) (fs.FileInfo, error),
	resolve func(string) (string, error), same func(a, b fs.FileInfo) bool) error {
	if err := checkOwner(name, opened); err != nil {
		return err
	}
	abs, err := filepath.Abs(name)
	if err != nil {
		return err
	}
	resolved, err := resolve(abs)
	if err != nil {
		return err
	}
	if fi, err := stat(resolved); err != nil {
		return err
	} else if !same(fi, opened) {
		return errors.New("the config file changed while it was being opened")
	}

	checked := map[string]bool{}
	for _, start := range []string{resolved, abs} {
		for dir := filepath.Dir(start); !checked[dir]; dir = filepath.Dir(dir) {
			checked[dir] = true
			fi, err := stat(dir)
			if err != nil {
				return err
			}
			if err := checkOwner(dir, fi); err != nil {
				return err
			}
			if filepath.Dir(dir) == dir {
				break // reached the root
			}
		}
	}
	return nil
}

func checkOwner(p string, fi fs.FileInfo) error {
	uid, ok := fileOwner(fi)
	if !ok {
		return nil // no ownership information on this platform
	}
	if uid != 0 {
		return fmt.Errorf("%s is owned by uid %d, not root; run: chown root %s", p, uid, p)
	}
	if fi.Mode().Perm()&0o022 != 0 {
		return fmt.Errorf("%s is writable by group or others (mode %04o); run: chmod go-w %s", p, fi.Mode().Perm(), p)
	}
	return nil
}
