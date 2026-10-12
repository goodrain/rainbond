package build

import (
	"reflect"
	"testing"
)

func TestDockerfileBuildArgsSupportsConsoleAndLegacyNames(t *testing.T) {
	envs := map[string]string{
		"BUILD_ARG_GOPROXY":         "https://goproxy.cn,direct",
		"ARG_GOPROXY":               "https://old.invalid",
		"ARG_LEGACY":                "legacy",
		"BUILD_ARG_TARGET_ARG_NAME": "preserved",
		"BUILD_ARG_EMPTY":           "",
		"BUILD_ARG_":                "invalid",
		"ARG_":                      "invalid",
		"GOPROXY":                   "runtime-only",
		"PROC_ENV":                  "not-a-build-argument",
	}
	want := []string{"--opt=build-arg:EMPTY=", "--opt=build-arg:GOPROXY=https://goproxy.cn,direct", "--opt=build-arg:LEGACY=legacy", "--opt=build-arg:TARGET_ARG_NAME=preserved"}
	if got := dockerfileBuildArgs(envs); !reflect.DeepEqual(got, want) {
		t.Fatalf("build arguments = %v; want %v", got, want)
	}
	if envs["ARG_GOPROXY"] != "https://old.invalid" {
		t.Fatal("input mutated")
	}
	if got := dockerfileBuildArgs(nil); len(got) != 0 {
		t.Fatal(got)
	}
}
