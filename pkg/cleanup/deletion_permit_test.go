package cleanup

import (
	"strings"
	"testing"
	"time"
)

// capability_id: rainbond.cleanup.registry-delete-permit
func TestRegistryDeletionPermitBindsOriginalTargetAndExpiry(t *testing.T) {
	key := []byte(strings.Repeat("fixture-only-", 4))
	now := time.Unix(1800000000, 0)
	request := operation("selected", "delete", "team/app")
	request.Target = "sha256:" + strings.Repeat("a", 64)
	permit, err := IssueRegistryDeletionPermit(key, request, now)
	if err != nil {
		t.Fatal(err)
	}
	verified, err := VerifyRegistryDeletionPermit(key, permit, now)
	if err != nil || verified != request {
		t.Fatal("permit did not retain original binding", err)
	}
	if _, err := VerifyRegistryDeletionPermit(key, permit, now.Add(6*time.Minute)); err == nil {
		t.Fatal("expired permit accepted")
	}
	if _, err := VerifyRegistryDeletionPermit(key, permit, now.Add(-time.Minute)); err == nil {
		t.Fatal("future permit accepted")
	}
	if _, err := VerifyRegistryDeletionPermit([]byte(strings.Repeat("different-fixture-", 3)), permit, now); err == nil {
		t.Fatal("foreign issuer accepted")
	}
	if _, err := VerifyRegistryDeletionPermit(key, "x"+permit, now); err == nil {
		t.Fatal("tampered permit accepted")
	}
	request.Target = "v1"
	if _, err := IssueRegistryDeletionPermit(key, request, now); err == nil {
		t.Fatal("mutable tag deletion authorized")
	}
}
