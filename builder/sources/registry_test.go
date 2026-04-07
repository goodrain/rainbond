package sources

import (
	"testing"

	"github.com/docker/distribution/reference"
)

// capability_id: rainbond.source-image.tag-from-ref
func TestGetTagFromNamedRef(t *testing.T) {
	named, err := reference.ParseNormalizedNamed("nginx:1.25")
	if err != nil {
		t.Fatal(err)
	}
	if got := GetTagFromNamedRef(named); got != "1.25" {
		t.Fatalf("expected tag 1.25, got %q", got)
	}

	untagged, err := reference.ParseNormalizedNamed("nginx")
	if err != nil {
		t.Fatal(err)
	}
	if got := GetTagFromNamedRef(untagged); got != "latest" {
		t.Fatalf("expected tag latest, got %q", got)
	}
}
