package fuzzy

import "testing"

// capability_id: rainbond.util.fuzzy.match
func TestMatch(t *testing.T) {
	if !Match("rbd", "rainbond-dashboard") {
		t.Fatal("expected fuzzy match")
	}
	if Match("zzz", "rainbond-dashboard") {
		t.Fatal("did not expect unrelated fuzzy match")
	}
}

// capability_id: rainbond.util.fuzzy.match-fold
func TestMatchFold(t *testing.T) {
	if !MatchFold("rbd", "RainBondDashboard") {
		t.Fatal("expected case-insensitive fuzzy match")
	}
	if MatchFold("zzz", "RainBondDashboard") {
		t.Fatal("did not expect unrelated fuzzy match")
	}
}

// capability_id: rainbond.util.fuzzy.rank-match
func TestRankMatch(t *testing.T) {
	if got := RankMatch("abc", "a-b-c"); got <= 0 {
		t.Fatalf("expected positive distance for fuzzy gap, got %d", got)
	}
	if got := RankMatch("abc", "xyz"); got != -1 {
		t.Fatalf("expected -1 for non-match, got %d", got)
	}
}

// capability_id: rainbond.util.fuzzy.find
func TestFind(t *testing.T) {
	got := Find("api", []string{"api-service", "dashboard", "service-api"})
	if len(got) != 2 || got[0] != "api-service" || got[1] != "service-api" {
		t.Fatalf("unexpected matches: %#v", got)
	}
}

// capability_id: rainbond.util.fuzzy.find-fold
func TestFindFold(t *testing.T) {
	got := FindFold("rbd", []string{"RainBondDashboard", "demo", "RBD-API"})
	if len(got) != 2 || got[0] != "RainBondDashboard" || got[1] != "RBD-API" {
		t.Fatalf("unexpected fold matches: %#v", got)
	}
}

// capability_id: rainbond.util.fuzzy.rank-find
func TestRankFind(t *testing.T) {
	got := RankFind("api", []string{"api", "service-api", "dashboard"})
	if len(got) != 2 {
		t.Fatalf("unexpected rank results: %#v", got)
	}
	if got[0].Distance > got[1].Distance {
		t.Fatalf("expected ascending distance ordering, got %#v", got)
	}
}

// capability_id: rainbond.util.fuzzy.rank-find-fold
func TestRankFindFold(t *testing.T) {
	got := RankFindFold("api", []string{"RBD-API", "Application", "demo"})
	if len(got) != 2 {
		t.Fatalf("unexpected rank results: %#v", got)
	}
	if got[0].Distance > got[1].Distance {
		t.Fatalf("expected ascending distance ordering, got %#v", got)
	}
}
