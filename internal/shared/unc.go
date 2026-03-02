package shared

import (
	"fmt"
	"strings"
)

// ValidateUNCPath checks that path is a UNC path and starts with one of the
// allowed share prefixes. Returns an error if either check fails.
func ValidateUNCPath(path string, allowedShares []string) error {
	if !strings.HasPrefix(path, `\\`) {
		return fmt.Errorf("path %q is not a UNC path", path)
	}
	lower := strings.ToLower(path)
	for _, prefix := range allowedShares {
		if strings.HasPrefix(lower, strings.ToLower(prefix)) {
			return nil
		}
	}
	return fmt.Errorf("path %q not in allowed shares", path)
}

// IsUNCPath returns true if path begins with \\.
func IsUNCPath(path string) bool {
	return strings.HasPrefix(path, `\\`)
}
