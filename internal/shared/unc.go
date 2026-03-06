package shared

import (
	"fmt"
	"strings"
)

// IsSharePath returns true if path is a valid share path — either a Windows
// UNC path (\\server\share\...) or a POSIX absolute path (/mnt/nas/...) used
// for NFS mounts on Linux agents.
func IsSharePath(path string) bool {
	return strings.HasPrefix(path, `\\`) || strings.HasPrefix(path, "/")
}

// ValidateSharePath checks that path is a valid share path and starts with one
// of the allowed share prefixes. It accepts Windows UNC paths (\\server\share)
// and POSIX absolute paths (/mnt/nas/media) for NFS mounts. Returns an error
// if either check fails.
func ValidateSharePath(path string, allowedShares []string) error {
	if !IsSharePath(path) {
		return fmt.Errorf("path %q is not a valid share path (expected \\\\server\\share\\... or /mnt/...)", path)
	}
	// UNC paths are case-insensitive (Windows); POSIX paths are case-sensitive.
	// Use case-insensitive matching for UNC, exact prefix for POSIX.
	isUNC := strings.HasPrefix(path, `\\`)
	for _, prefix := range allowedShares {
		if isUNC {
			if strings.HasPrefix(strings.ToLower(path), strings.ToLower(prefix)) {
				return nil
			}
		} else {
			if strings.HasPrefix(path, prefix) {
				return nil
			}
		}
	}
	return fmt.Errorf("path %q not in allowed shares", path)
}

// IsUNCPath returns true if path begins with \\.
// Deprecated: use IsSharePath which also accepts POSIX NFS mount paths.
func IsUNCPath(path string) bool {
	return strings.HasPrefix(path, `\\`)
}
