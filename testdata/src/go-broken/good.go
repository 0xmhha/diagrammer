// Package broken is a fixture whose point is that one of its files does not
// parse. The graph must still hold this file, and must say the other one is
// missing rather than leaving no trace of it.
package broken

// Good parses.
func Good() string {
	return "good"
}
