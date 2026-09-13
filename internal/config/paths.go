package config

import (
	"os"
	"path/filepath"
)

// ResolvePath selects an absolute configuration path without creating anything.
func ResolvePath(explicit string) (string, error) {
	if explicit != "" {
		path, err := filepath.Abs(explicit)
		if err != nil {
			return "", &Error{Code: ERR_CONFIG_INVALID, Stage: "path", Reason: "absolute path unavailable", Cause: err}
		}
		return path, nil
	}
	if base := os.Getenv("XDG_CONFIG_HOME"); filepath.IsAbs(base) {
		return filepath.Join(base, "azerlay", "config.toml"), nil
	}
	if home := os.Getenv("HOME"); filepath.IsAbs(home) {
		return filepath.Join(home, ".config", "azerlay", "config.toml"), nil
	}
	return "", &Error{Code: ERR_CONFIG_INVALID, Stage: "path", Reason: "absolute configuration base unavailable"}
}
