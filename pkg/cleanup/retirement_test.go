package cleanup

import (
	"errors"
	"testing"
)

func TestRetirementProtectsLiveOrChangedVersions(t *testing.T) {
	base := VersionState{EventID: "build-event", ServiceID: "service", Version: "old", CurrentVersion: "current", Image: "registry/app@sha256:old", Status: "success", DeliveredType: "image"}
	expected := VersionExpectation{EventID: "build-event", ServiceID: "service", Version: "old", CurrentVersion: "current", Image: base.Image}
	if err := ValidateVersionRetirement(base, expected); err != nil {
		t.Fatal(err)
	}
	cases := []VersionState{base, base, base, base, base}
	cases[0].CurrentVersion = "old"
	cases[1].ActiveOperation = true
	cases[2].Image = "registry/changed"
	cases[3].DeliveredType = "slug"
	cases[4].ServiceID = "other"
	for _, state := range cases {
		if err := ValidateVersionRetirement(state, expected); err == nil {
			t.Fatal("protected or changed version accepted")
		}
	}
	expected.CurrentVersion = "stale"
	if !errors.Is(ValidateVersionRetirement(base, expected), ErrStateChanged) {
		t.Fatal("stale plan accepted")
	}
}
