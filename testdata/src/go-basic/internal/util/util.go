// Package util holds one helper, so that internal/ becomes a grouping
// directory with no Go files of its own.
package util

import "strings"

// Normalize lowercases and trims an identifier.
func Normalize(id string) string {
	return strings.ToLower(strings.TrimSpace(id))
}
