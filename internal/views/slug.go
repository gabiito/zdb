package views

import (
	"regexp"
	"strings"
)

// slugSubstRe matches any run of characters that are not lowercase ASCII
// alphanumerics, dots, underscores, or hyphens.
var slugSubstRe = regexp.MustCompile(`[^a-z0-9._-]+`)

// Slug normalises a connection name into a filesystem-safe directory name.
// The algorithm:
//  1. Lowercase the entire string.
//  2. Replace every contiguous run of characters not matching [a-z0-9._-] with
//     a single hyphen.
//  3. Trim leading and trailing hyphens.
//
// Returns the empty string when no allowed character survives normalisation.
// Callers MUST treat an empty return value as a validation error (REQ-3).
func Slug(name string) string {
	s := strings.ToLower(name)
	s = slugSubstRe.ReplaceAllString(s, "-")
	s = strings.Trim(s, "-")
	return s
}
