//go:build !unix

package config

import "io/fs"

// checkTrusted is a no-op where files have no Unix owner. gluper only
// requires a trusted config file when running as root, which never happens
// on Windows (the effective UID there is -1).
func checkTrusted(string, fs.FileInfo) error { return nil }
