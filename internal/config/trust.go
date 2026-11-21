package config

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
)

// checkTrusted refuses a config file that a user other than root could have
// written: the file and every directory holding it (before and after
// resolving symlinks) must be owned by root and not writable by group or
// others. gluper runs hooks as root, so a writable config would let another
// user run commands as root.
func checkTrusted(name string) error {
	return checkTrustedWith(name, os.Stat, filepath.EvalSymlinks)
}

func checkTrustedWith(name string, stat func(string) (fs.FileInfo, error), resolve func(string) (string, error)) error {
	abs, err := filepath.Abs(name)
	if err != nil {
		return err
	}
	resolved, err := resolve(abs)
	if err != nil {
		return err
	}
	paths := []string{resolved, filepath.Dir(resolved)}
	if resolved != abs {
		paths = append(paths, filepath.Dir(abs))
	}
	for _, p := range paths {
		fi, err := stat(p)
		if err != nil {
			return err
		}
		uid, ok := fileOwner(fi)
		if !ok {
			continue // no ownership information on this platform
		}
		if uid != 0 {
			return fmt.Errorf("%s is owned by uid %d, not root; run: chown root %s", p, uid, p)
		}
		if fi.Mode().Perm()&0o022 != 0 {
			return fmt.Errorf("%s is writable by group or others (mode %04o); run: chmod go-w %s", p, fi.Mode().Perm(), p)
		}
	}
	return nil
}
