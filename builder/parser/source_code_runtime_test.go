package parser

import (
	"testing"

	"github.com/goodrain/rainbond/builder/parser/types"
)

func TestApplyRuntimeBuildEnvsCNBMapsStartCommandToAutoProcfileWithoutExposingSourceEnv(t *testing.T) {
	envs := map[string]*types.Env{}
	runtimeInfo := map[string]string{
		"PACKAGE_TOOL":     "pip",
		"START_CMD":        "web: flask --app python_flask.app run --host 0.0.0.0 --port $_PORT",
		"START_CMD_SOURCE": "auto-detected",
		"RUNTIMES":         "3.14",
	}

	applyRuntimeBuildEnvs(envs, runtimeInfo, "cnb")

	if got := envs["BUILD_PACKAGE_TOOL"]; got == nil || got.Value != "pip" {
		t.Fatalf("expected BUILD_PACKAGE_TOOL=pip, got %#v", got)
	}
	if got := envs["BUILD_AUTO_PROCFILE"]; got == nil || got.Value != "web: flask --app python_flask.app run --host 0.0.0.0 --port $_PORT" {
		t.Fatalf("expected BUILD_AUTO_PROCFILE to be populated, got %#v", got)
	}
	if _, ok := envs["START_COMMAND_SOURCE"]; ok {
		t.Fatalf("expected START_COMMAND_SOURCE to remain internal runtime metadata, got %#v", envs["START_COMMAND_SOURCE"])
	}
	if got := envs["BUILD_RUNTIMES"]; got == nil || got.Value != "3.14" {
		t.Fatalf("expected BUILD_RUNTIMES=3.14, got %#v", got)
	}
	if _, ok := envs["BUILD_START_CMD"]; ok {
		t.Fatalf("expected BUILD_START_CMD to be omitted for cnb builds")
	}
}
