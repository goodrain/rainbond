package fuzzy

import "testing"

// capability_id: rainbond.util.fuzzy.levenshtein-distance
func TestLevenshteinDistance(t *testing.T) {
	if got := LevenshteinDistance("kitten", "sitting"); got != 3 {
		t.Fatalf("expected distance 3, got %d", got)
	}
	if got := LevenshteinDistance("rainbond", "rainbond"); got != 0 {
		t.Fatalf("expected distance 0, got %d", got)
	}
}
