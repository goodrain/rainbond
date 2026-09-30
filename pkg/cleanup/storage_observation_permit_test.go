package cleanup

import (
	"strings"
	"testing"
	"time"
)

func TestStorageObservationPermitBindsStoreFingerprintAndExpiry(t *testing.T) {
	key := []byte(strings.Repeat("observation-key-", 3))
	binding := StorageRegistration{StorageID: "store", Generation: "one", VolumeUID: "volume", RootPath: "/var/lib/registry"}
	now := time.Unix(1000, 0).UTC()
	token, err := IssueStorageObservationPermit(key, binding, now)
	if err != nil {
		t.Fatal(err)
	}
	observed, err := VerifyStorageObservationPermit(key, token, now.Add(30*time.Second))
	if err != nil || observed != binding {
		t.Fatal(observed, err)
	}
	for _, check := range []struct {
		key   []byte
		token string
		now   time.Time
	}{
		{[]byte(strings.Repeat("different-key-", 3)), token, now},
		{key, token + "x", now},
		{key, token, now.Add(time.Minute)},
	} {
		if _, err := VerifyStorageObservationPermit(check.key, check.token, check.now); err == nil {
			t.Fatal("changed/expired observation permit accepted")
		}
	}
}
