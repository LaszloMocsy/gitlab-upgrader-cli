package config

import "strings"

// suggest returns the most likely intended key for an unknown name inside
// section prefix, or "". It first tries the names in the same section, then
// any key with exactly that name elsewhere (a key placed in the wrong
// section).
func suggest(prefix, name string) string {
	best, bestDist := "", maxDistance(name)+1
	for _, cand := range sections[prefix] {
		if d := levenshtein(strings.ToLower(name), cand); d < bestDist {
			best, bestDist = joinPath(prefix, cand), d
		}
	}
	if best != "" {
		return best
	}
	for _, k := range allKeys {
		if k.Path[strings.LastIndex(k.Path, ".")+1:] == name {
			return k.Path
		}
	}
	for s := range sections {
		if s != "" && s[strings.LastIndex(s, ".")+1:] == name {
			return s
		}
	}
	return ""
}

// suggestEnv returns the closest known GLUPER_* variable to name, or "".
func suggestEnv(name string) string {
	best, bestDist := "", maxDistance(name)+1
	for _, k := range allKeys {
		if d := levenshtein(name, k.Env); d < bestDist {
			best, bestDist = k.Env, d
		}
	}
	return best
}

func maxDistance(s string) int {
	return min(3, max(1, len(s)/4))
}

func levenshtein(a, b string) int {
	ra, rb := []rune(a), []rune(b)
	prev := make([]int, len(rb)+1)
	cur := make([]int, len(rb)+1)
	for j := range prev {
		prev[j] = j
	}
	for i := 1; i <= len(ra); i++ {
		cur[0] = i
		for j := 1; j <= len(rb); j++ {
			cost := 1
			if ra[i-1] == rb[j-1] {
				cost = 0
			}
			cur[j] = min(prev[j]+1, cur[j-1]+1, prev[j-1]+cost)
		}
		prev, cur = cur, prev
	}
	return prev[len(rb)]
}
