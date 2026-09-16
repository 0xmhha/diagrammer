// Package basic is a fixture. It exists so the analyzer has a tree whose shape
// is known in advance: a group directory, several packages, exported and
// unexported declarations, and edges that cross package boundaries.
package basic

// Version is exported, so the analyzer must mark it as such.
const Version = "1"

// hidden is not exported, and must not be marked as such.
const hidden = "1"
