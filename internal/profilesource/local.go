package profilesource

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
)

func validLocalComponent(value string) bool {
	return value != "" && value != "." && value != ".." && !filepath.IsAbs(value) && !strings.ContainsAny(value, "/\\\x00")
}

func validLocalRef(ref SourceRef) bool {
	return ref.Hash == "" && filepath.IsAbs(ref.Local.Root) && !strings.ContainsRune(ref.Local.Root, 0) && validLocalComponent(ref.Local.Device) && validLocalComponent(ref.Local.File)
}

func localRelativePath(ref LocalRef) string {
	return filepath.Join("Storage", "DevicesStorage", ref.Device, "ProfileStorage", ref.File)
}

// localRoots resolves configuration once, without expanding or creating paths.
func localRoots(explicit string) ([]string, error) {
	if explicit != "" {
		root, err := filepath.Abs(explicit)
		if err != nil || strings.ContainsRune(root, 0) {
			return nil, &Error{Code: ERR_PROFILE_STORAGE, Cause: errUnsafePath}
		}
		return []string{root}, nil
	}
	var roots []string
	if config := os.Getenv("XDG_CONFIG_HOME"); filepath.IsAbs(config) {
		roots = append(roots, filepath.Join(config, "Azeron Software"))
	}
	if home := os.Getenv("HOME"); filepath.IsAbs(home) {
		candidate := filepath.Join(home, ".config", "Azeron Software")
		duplicate := false
		for _, root := range roots {
			left, leftErr := os.Stat(root)
			right, rightErr := os.Stat(candidate)
			if root == candidate || leftErr == nil && rightErr == nil && os.SameFile(left, right) {
				duplicate = true
			}
		}
		if !duplicate {
			roots = append(roots, candidate)
		}
	}
	if len(roots) == 0 {
		return nil, &Error{Code: ERR_PROFILE_STORAGE, Cause: errUnsafePath}
	}
	return roots, nil
}

// ResolveLocalRef matches an exact configured selection. Admission and stable
// reading remain the provider's responsibility; detection never chooses a file.
func ResolveLocalRef(explicit, device, file string) (LocalRef, error) {
	if !validLocalComponent(device) || !validLocalComponent(file) {
		return LocalRef{}, &Error{Code: ERR_PROFILE_NOT_FOUND, Cause: errInvalidSelection}
	}
	roots, err := localRoots(explicit)
	if err != nil {
		return LocalRef{}, err
	}
	// An explicit root fixes identity even when its selected file is unavailable.
	if explicit != "" {
		return LocalRef{Root: roots[0], Device: device, File: file}, nil
	}
	var selected LocalRef
	for _, root := range roots {
		ref := LocalRef{Root: root, Device: device, File: file}
		_, err := os.Stat(filepath.Join(root, localRelativePath(ref)))
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			return LocalRef{}, &Error{Code: ERR_PROFILE_STORAGE, Cause: err}
		}
		if selected != (LocalRef{}) {
			return LocalRef{}, &Error{Code: ERR_PROFILE_STORAGE, Cause: errInvalidSelection}
		}
		selected = ref
	}
	if selected == (LocalRef{}) {
		return LocalRef{}, &Error{Code: ERR_PROFILE_NOT_FOUND}
	}
	return selected, nil
}
