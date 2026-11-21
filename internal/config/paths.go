package config

import (
	"errors"
	"path/filepath"
)

// SystemPath is the config file used when gluper runs as root on Linux.
const SystemPath = "/etc/gluper/config.yaml"

// DefaultPath returns the config file gluper reads when none is named:
// SystemPath for root on Linux, otherwise gluper/config.yaml inside the
// user's config directory (os.UserConfigDir).
func DefaultPath(goos string, euid int, userConfigDir func() (string, error)) (string, error) {
	if goos == "linux" && euid == 0 {
		return SystemPath, nil
	}
	dir, err := userConfigDir()
	if err != nil {
		return "", err
	}
	if dir == "" {
		return "", errors.New("no user config directory")
	}
	return filepath.Join(dir, "gluper", "config.yaml"), nil
}
