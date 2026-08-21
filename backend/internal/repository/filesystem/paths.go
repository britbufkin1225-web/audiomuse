package filesystem

import (
	"errors"
	"path"
	"strings"
)

// errUnsafePath marks a repository-relative locator that must never be joined to the
// corpus root.
var errUnsafePath = errors.New("unsafe repository-relative path")

// safeRelPath normalises a repository-relative locator and rejects anything that could
// escape the corpus root.
//
// Canonical locators are repository-relative slash paths. Absolute paths, drive letters,
// UNC prefixes, parent traversal and embedded NUL bytes are all rejected rather than
// cleaned, because a locator that needs cleaning is an authoring defect worth reporting.
func safeRelPath(raw string) (string, error) {
	if raw == "" {
		return "", errUnsafePath
	}
	if strings.ContainsRune(raw, 0) {
		return "", errUnsafePath
	}
	// Windows-authored locators may use backslashes; normalise before inspection.
	normalized := strings.ReplaceAll(raw, `\`, "/")
	if strings.HasPrefix(normalized, "/") || strings.HasPrefix(normalized, "//") {
		return "", errUnsafePath
	}
	// Reject "C:/..." style volume-qualified paths.
	if len(normalized) >= 2 && normalized[1] == ':' {
		return "", errUnsafePath
	}
	cleaned := path.Clean(normalized)
	if cleaned == ".." || strings.HasPrefix(cleaned, "../") || cleaned == "." {
		return "", errUnsafePath
	}
	return cleaned, nil
}

// isExternalLocator reports whether a registry locator names something outside the
// repository. The canonical registry stores external references in notes and keeps the
// locator repository-relative, so this is a guard rather than an expected case.
func isExternalLocator(raw string) bool {
	lowered := strings.ToLower(raw)
	return strings.HasPrefix(lowered, "http://") ||
		strings.HasPrefix(lowered, "https://") ||
		strings.HasPrefix(lowered, "mailto:")
}
