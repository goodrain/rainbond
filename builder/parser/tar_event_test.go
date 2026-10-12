package parser

import "testing"

// capability_id: rainbond.cleanup.tar-check-event-path
func TestTarImageEventIDRejectsTraversalAndMalformedSources(t *testing.T) {
	for _, source := range []string{"event", " event id", "event ../outside", "event /root", "event id extra", "events id", "event a/b"} {
		if _, ok := TarImageEventID(source); ok {
			t.Fatal("unsafe event source accepted", source)
		}
	}
	id, ok := TarImageEventID("event 0123456789abcdef0123456789abcdef")
	if !ok || id != "0123456789abcdef0123456789abcdef" {
		t.Fatal(id, ok)
	}
}
