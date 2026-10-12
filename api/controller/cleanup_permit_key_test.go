package controller

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

// capability_id: rainbond.cleanup.registry-permit-key-separation
func TestRegistryPermitKeyUsesDedicatedFileAndNeverFallsBackOnFailure(t *testing.T) {
	legacy := bytes.Repeat([]byte("l"), 32)
	dedicated := bytes.Repeat([]byte("p"), 32)
	t.Setenv("TOKEN", string(legacy))
	t.Setenv("CLEANUP_REGISTRY_PERMIT_KEY_FILE", "")
	if !bytes.Equal(systemRegistryPermitKey(), legacy) {
		t.Fatal("lost explicit legacy compatibility")
	}
	path := filepath.Join(t.TempDir(), "permit")
	if err := os.WriteFile(path, dedicated, 0600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("CLEANUP_REGISTRY_PERMIT_KEY_FILE", path)
	if !bytes.Equal(systemRegistryPermitKey(), dedicated) {
		t.Fatal("did not use independent permit key")
	}
	for _, bad := range [][]byte{[]byte("short"), bytes.Repeat([]byte("x"), 8193), append(bytes.Repeat([]byte("x"), 32), ' ', 'x')} {
		if err := os.WriteFile(path, bad, 0600); err != nil {
			t.Fatal(err)
		}
		if len(systemRegistryPermitKey()) != 0 {
			t.Fatal("invalid configured key fell back to administrator token")
		}
	}
	t.Setenv("CLEANUP_REGISTRY_PERMIT_KEY_FILE", path+".missing")
	if len(systemRegistryPermitKey()) != 0 {
		t.Fatal("missing configured key fell back")
	}
}
