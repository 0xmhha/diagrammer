package broken

// Bad does not parse: the parameter list is never closed. A soft-failing
// parser would drop this function and leave the graph looking complete, which
// is the failure the diagnostics block exists to make visible.
func Bad(a int {
	return
}
