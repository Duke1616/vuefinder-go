package finder

import (
	"errors"
	"fmt"
	"strings"
)

// Common, backend-agnostic errors (wrap backend-specific errors with these)
var (
	ErrNotFound      = errors.New("not found")
	ErrAlreadyExists = errors.New("already exists")
	ErrPermission    = errors.New("permission denied")
	ErrConflict      = errors.New("conflict")
	ErrTransient     = errors.New("transient error")
	ErrInvalidLoc    = errors.New("invalid locator")
)

// Locator is a unified resource identifier, like sftp:///path, s3://bucket/key, fs:///abs/path
// Scheme determines the backend; Opaque contains the path/bucket+key part.
type Locator struct {
	Scheme string
	Opaque string
}

func (l Locator) String() string {
	if l.Scheme == "" {
		return l.Opaque
	}
	return l.Scheme + "://" + l.Opaque
}

// ParseLocator parses a string like "sftp:///home/user/file" into a Locator.
func ParseLocator(s string) (Locator, error) {
	if s == "" {
		return Locator{}, fmt.Errorf("%w: empty", ErrInvalidLoc)
	}
	parts := strings.SplitN(s, "://", 2)
	if len(parts) == 1 {
		// no scheme, treat as opaque path
		return Locator{Opaque: normalizeLocatorPath(parts[0])}, nil
	}
	scheme := strings.ToLower(parts[0])
	opaque := parts[1]
	if scheme == "" {
		return Locator{}, fmt.Errorf("%w: missing scheme in %q", ErrInvalidLoc, s)
	}
	return Locator{Scheme: scheme, Opaque: normalizeLocatorPath(opaque)}, nil
}

// normalizeLocatorPath trims duplicate slashes but keeps leading slash when present.
func normalizeLocatorPath(p string) string {
	if p == "" {
		return "/"
	}
	// collapse multiple slashes
	for strings.Contains(p, "//") {
		p = strings.ReplaceAll(p, "//", "/")
	}
	if strings.HasPrefix(p, "/") {
		return p
	}
	return "/" + p
}
