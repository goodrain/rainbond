package build

import "testing"

func TestExpandBuildEnvsForSlugBuildPreservesBuildKeysAndAddsLegacyAliases(t *testing.T) {
	envs := map[string]string{
		"BUILD_PROCFILE":               "web: flask --app demo.app run --host 0.0.0.0 --port $PORT",
		"BUILD_NO_CACHE":               "true",
		"BUILD_RUNTIMES":               "3.14",
		"BUILD_RUNTIMES_SERVER":        "nginx",
		"BUILD_RUNTIMES_MAVEN":         "3.9.14",
		"BUILD_GOVERSION":              "1.25",
		"BUILD_PYTHON_PACKAGE_MANAGER": "pip",
	}

	got := expandBuildEnvsForSlugBuild(envs)

	if got["BUILD_PROCFILE"] != envs["BUILD_PROCFILE"] {
		t.Fatalf("expected BUILD_PROCFILE to be preserved, got %q", got["BUILD_PROCFILE"])
	}
	if got["PROCFILE"] != envs["BUILD_PROCFILE"] {
		t.Fatalf("expected PROCFILE alias to be synthesized, got %q", got["PROCFILE"])
	}
	if got["NO_CACHE"] != envs["BUILD_NO_CACHE"] {
		t.Fatalf("expected NO_CACHE alias to be synthesized, got %q", got["NO_CACHE"])
	}
	if got["RUNTIMES"] != envs["BUILD_RUNTIMES"] {
		t.Fatalf("expected RUNTIMES alias to be synthesized, got %q", got["RUNTIMES"])
	}
	if got["RUNTIMES_SERVER"] != envs["BUILD_RUNTIMES_SERVER"] {
		t.Fatalf("expected RUNTIMES_SERVER alias to be synthesized, got %q", got["RUNTIMES_SERVER"])
	}
	if got["RUNTIMES_MAVEN"] != envs["BUILD_RUNTIMES_MAVEN"] {
		t.Fatalf("expected RUNTIMES_MAVEN alias to be synthesized, got %q", got["RUNTIMES_MAVEN"])
	}
	if got["GOVERSION"] != envs["BUILD_GOVERSION"] {
		t.Fatalf("expected GOVERSION alias to be synthesized, got %q", got["GOVERSION"])
	}
	if _, ok := got["PYTHON_PACKAGE_MANAGER"]; ok {
		t.Fatalf("did not expect BUILD_PYTHON_PACKAGE_MANAGER to be converted for slug builds")
	}
}

func TestExpandBuildEnvsForSlugBuildDoesNotOverrideExistingLegacyAliases(t *testing.T) {
	envs := map[string]string{
		"BUILD_PROCFILE": "web: gunicorn demo.wsgi:application --bind 0.0.0.0:$PORT",
		"PROCFILE":       "web: python app.py",
	}

	got := expandBuildEnvsForSlugBuild(envs)

	if got["PROCFILE"] != "web: python app.py" {
		t.Fatalf("expected explicit PROCFILE to win, got %q", got["PROCFILE"])
	}
}
