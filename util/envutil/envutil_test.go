package envutil

import "testing"

// capability_id: rainbond.envutil.memory-label
func TestGetMemoryType(t *testing.T) {
	if got := GetMemoryType(128); got != "micro" {
		t.Fatalf("expected micro, got %q", got)
	}
	if got := GetMemoryType(999); got != "small" {
		t.Fatalf("expected default small, got %q", got)
	}
}

// capability_id: rainbond.envutil.custom-memory
func TestIsCustomMemory(t *testing.T) {
	if IsCustomMemory(128) {
		t.Fatal("did not expect predefined memory to be custom")
	}
	if !IsCustomMemory(999) {
		t.Fatal("expected unknown memory size to be custom")
	}
}

// capability_id: rainbond.envutil.getenv-default
func TestGetenvDefault(t *testing.T) {
	t.Setenv("RBD_ENVUTIL_TEST", "")
	if got := GetenvDefault("RBD_ENVUTIL_TEST", "fallback"); got != "fallback" {
		t.Fatalf("expected fallback, got %q", got)
	}
	t.Setenv("RBD_ENVUTIL_TEST", "value")
	if got := GetenvDefault("RBD_ENVUTIL_TEST", "fallback"); got != "value" {
		t.Fatalf("expected explicit env value, got %q", got)
	}
}
