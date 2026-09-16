package compose

import "sort"

// orderByAdjacency arranges ids so that things joined by a line end up near
// each other on the grid.
//
// Placement used to follow sorted ids, which ignores the relationships
// entirely. Two components that talk to each other constantly could land at
// opposite corners, and the route between them then had to cross the whole
// page: a long detour that consumes channel lanes, crosses other routes and
// takes its label past everything in between. Measured on the fixtures, that
// single decision was the largest cause of relationships the drawing could not
// hold.
//
// The arrangement is a breadth-first walk from the best-connected id, so a
// cluster comes out as a contiguous run and separate clusters do not interleave.
// It is not an optimal layout — that is a hard problem and nothing here needs
// one — it just stops the obvious waste.
//
// Ties break on id at every step, so the same model always produces the same
// arrangement. The rules that decide what to draw are the contract; this decides
// only where things go, and is free to improve.
func orderByAdjacency(ids []string, edges [][2]string, keyOf func(string) string) []string {
	neighbours := make(map[string][]string, len(ids))
	present := make(map[string]bool, len(ids))
	for _, id := range ids {
		present[id] = true
	}
	for _, e := range edges {
		from, to := e[0], e[1]
		if !present[from] || !present[to] || from == to {
			continue
		}
		neighbours[from] = append(neighbours[from], to)
		neighbours[to] = append(neighbours[to], from)
	}
	for id := range neighbours {
		sort.Strings(neighbours[id])
	}

	// Candidates are visited best-connected first, and by id within a degree,
	// so the densest cluster is laid down before the stragglers.
	candidates := append([]string(nil), ids...)
	sort.Slice(candidates, func(i, j int) bool {
		di, dj := len(neighbours[candidates[i]]), len(neighbours[candidates[j]])
		if di != dj {
			return di > dj
		}
		return candidates[i] < candidates[j]
	})

	seen := make(map[string]bool, len(ids))
	out := make([]string, 0, len(ids))
	for _, start := range candidates {
		if seen[start] {
			continue
		}
		// A group the caller wants kept together — a region band, say — is
		// never split by the walk: its members are emitted with it.
		queue := []string{start}
		seen[start] = true
		for len(queue) > 0 {
			id := queue[0]
			queue = queue[1:]
			out = append(out, id)
			for _, next := range neighbours[id] {
				if !seen[next] {
					seen[next] = true
					queue = append(queue, next)
				}
			}
		}
	}

	// Grouping wins over adjacency where the two disagree, because a band drawn
	// around a scatter is not a band.
	if keyOf != nil {
		sort.SliceStable(out, func(i, j int) bool { return keyOf(out[i]) < keyOf(out[j]) })
	}
	return out
}
