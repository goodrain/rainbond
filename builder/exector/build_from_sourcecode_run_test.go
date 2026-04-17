package exector

import "testing"

func TestSourceBuildModeFallsBackToBuildMode(t *testing.T) {
	envs := map[string]string{
		"BUILD_MODE": "DOCKERFILE",
	}

	if got := sourceBuildMode(envs); got != "DOCKERFILE" {
		t.Fatalf("expected sourceBuildMode to use BUILD_MODE fallback, got %q", got)
	}
}

func TestSourceBuildModePrefersExplicitMode(t *testing.T) {
	envs := map[string]string{
		"MODE":       "default",
		"BUILD_MODE": "DOCKERFILE",
	}

	if got := sourceBuildMode(envs); got != "DEFAULT" {
		t.Fatalf("expected MODE to win over BUILD_MODE, got %q", got)
	}
}

func TestSourceBuildNoCacheEnabledUsesBuildAlias(t *testing.T) {
	envs := map[string]string{
		"BUILD_NO_CACHE": "True",
	}

	if !sourceBuildNoCacheEnabled(envs) {
		t.Fatal("expected BUILD_NO_CACHE to enable no-cache mode")
	}
}

func TestSourceBuildNoCacheEnabledIgnoresBlankLegacyKey(t *testing.T) {
	envs := map[string]string{
		"NO_CACHE":       "",
		"BUILD_NO_CACHE": "true",
	}

	if !sourceBuildNoCacheEnabled(envs) {
		t.Fatal("expected BUILD_NO_CACHE to win when NO_CACHE is blank")
	}
}
