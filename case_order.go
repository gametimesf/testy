package testy

// Reject the whole hint rather than dropping, duplicating, or accidentally
// starving cases when persisted history and the current registry disagree.
func caseDispatchOrder(cases []testCase, preferred []string) ([]int, bool) {
	canonical := make([]int, len(cases))
	byName := make(map[string]int, len(cases))
	for i, c := range cases {
		canonical[i] = i
		byName[c.Name] = i
	}
	if len(preferred) == 0 {
		return canonical, true
	}
	if len(preferred) != len(cases) {
		return canonical, false
	}
	order := make([]int, len(cases))
	seen := make([]bool, len(cases))
	for i, name := range preferred {
		index, ok := byName[name]
		if !ok || seen[index] {
			return canonical, false
		}
		seen[index] = true
		order[i] = index
	}
	return order, true
}
