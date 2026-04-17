package build

import "testing"

func TestExpandBuildEnvsForSlugBuildPreservesBuildKeysAndAddsLegacyAliases(t *testing.T) {
	envs := map[string]string{
		"BUILD_PROCFILE":                "web: flask --app demo.app run --host 0.0.0.0 --port $PORT",
		"BUILD_NO_CACHE":                "true",
		"BUILD_ENABLE_ORACLEJDK":        "True",
		"BUILD_ORACLEJDK_URL":           "https://example.com/jdk.tar.gz",
		"BUILD_MAVEN_SETTING_NAME":      "tencent",
		"BUILD_MAVEN_CUSTOM_OPTS":       "-DskipTests",
		"BUILD_MAVEN_CUSTOM_GOALS":      "clean package",
		"BUILD_MAVEN_JAVA_OPTS":         "-Xmx1024m",
		"BUILD_MAVEN_MIRROR_DISABLE":    "true",
		"BUILD_MAVEN_MIRROR_OF":         "central",
		"BUILD_MAVEN_MIRROR_URL":        "https://mirror.example.com",
		"BUILD_GOPROXY":                 "https://goproxy.cn",
		"BUILD_GOPRIVATE":               "git.example.com",
		"BUILD_GO_INSTALL_PACKAGE_SPEC": "./cmd/demo",
		"BUILD_PIP_INDEX_URL":           "https://pypi.example.com/simple",
		"BUILD_NODE_ENV":                "production",
		"BUILD_NODE_MODULES_CACHE":      "true",
		"BUILD_NODE_BUILD_CMD":          "npm run build",
		"BUILD_NPM_REGISTRY":            "https://registry.npmmirror.com",
		"BUILD_YARN_REGISTRY":           "https://registry.yarnpkg.com",
		"BUILD_RUNTIMES":                "3.14",
		"BUILD_RUNTIMES_SERVER":         "nginx",
		"BUILD_RUNTIMES_MAVEN":          "3.9.14",
		"BUILD_GOVERSION":               "1.25",
		"BUILD_PYTHON_PACKAGE_MANAGER":  "pip",
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
	if got["ENABLE_ORACLEJDK"] != envs["BUILD_ENABLE_ORACLEJDK"] {
		t.Fatalf("expected ENABLE_ORACLEJDK alias to be synthesized, got %q", got["ENABLE_ORACLEJDK"])
	}
	if got["ORACLEJDK_URL"] != envs["BUILD_ORACLEJDK_URL"] {
		t.Fatalf("expected ORACLEJDK_URL alias to be synthesized, got %q", got["ORACLEJDK_URL"])
	}
	if got["MAVEN_SETTING_NAME"] != envs["BUILD_MAVEN_SETTING_NAME"] {
		t.Fatalf("expected MAVEN_SETTING_NAME alias to be synthesized, got %q", got["MAVEN_SETTING_NAME"])
	}
	if got["MAVEN_CUSTOM_OPTS"] != envs["BUILD_MAVEN_CUSTOM_OPTS"] {
		t.Fatalf("expected MAVEN_CUSTOM_OPTS alias to be synthesized, got %q", got["MAVEN_CUSTOM_OPTS"])
	}
	if got["MAVEN_CUSTOM_GOALS"] != envs["BUILD_MAVEN_CUSTOM_GOALS"] {
		t.Fatalf("expected MAVEN_CUSTOM_GOALS alias to be synthesized, got %q", got["MAVEN_CUSTOM_GOALS"])
	}
	if got["MAVEN_JAVA_OPTS"] != envs["BUILD_MAVEN_JAVA_OPTS"] {
		t.Fatalf("expected MAVEN_JAVA_OPTS alias to be synthesized, got %q", got["MAVEN_JAVA_OPTS"])
	}
	if got["MAVEN_MIRROR_DISABLE"] != envs["BUILD_MAVEN_MIRROR_DISABLE"] {
		t.Fatalf("expected MAVEN_MIRROR_DISABLE alias to be synthesized, got %q", got["MAVEN_MIRROR_DISABLE"])
	}
	if got["MAVEN_MIRROR_OF"] != envs["BUILD_MAVEN_MIRROR_OF"] {
		t.Fatalf("expected MAVEN_MIRROR_OF alias to be synthesized, got %q", got["MAVEN_MIRROR_OF"])
	}
	if got["MAVEN_MIRROR_URL"] != envs["BUILD_MAVEN_MIRROR_URL"] {
		t.Fatalf("expected MAVEN_MIRROR_URL alias to be synthesized, got %q", got["MAVEN_MIRROR_URL"])
	}
	if got["GOPROXY"] != envs["BUILD_GOPROXY"] {
		t.Fatalf("expected GOPROXY alias to be synthesized, got %q", got["GOPROXY"])
	}
	if got["GOPRIVATE"] != envs["BUILD_GOPRIVATE"] {
		t.Fatalf("expected GOPRIVATE alias to be synthesized, got %q", got["GOPRIVATE"])
	}
	if got["GO_INSTALL_PACKAGE_SPEC"] != envs["BUILD_GO_INSTALL_PACKAGE_SPEC"] {
		t.Fatalf("expected GO_INSTALL_PACKAGE_SPEC alias to be synthesized, got %q", got["GO_INSTALL_PACKAGE_SPEC"])
	}
	if got["PIP_INDEX_URL"] != envs["BUILD_PIP_INDEX_URL"] {
		t.Fatalf("expected PIP_INDEX_URL alias to be synthesized, got %q", got["PIP_INDEX_URL"])
	}
	if got["NODE_ENV"] != envs["BUILD_NODE_ENV"] {
		t.Fatalf("expected NODE_ENV alias to be synthesized, got %q", got["NODE_ENV"])
	}
	if got["NODE_MODULES_CACHE"] != envs["BUILD_NODE_MODULES_CACHE"] {
		t.Fatalf("expected NODE_MODULES_CACHE alias to be synthesized, got %q", got["NODE_MODULES_CACHE"])
	}
	if got["NODE_BUILD_CMD"] != envs["BUILD_NODE_BUILD_CMD"] {
		t.Fatalf("expected NODE_BUILD_CMD alias to be synthesized, got %q", got["NODE_BUILD_CMD"])
	}
	if got["NPM_REGISTRY"] != envs["BUILD_NPM_REGISTRY"] {
		t.Fatalf("expected NPM_REGISTRY alias to be synthesized, got %q", got["NPM_REGISTRY"])
	}
	if got["YARN_REGISTRY"] != envs["BUILD_YARN_REGISTRY"] {
		t.Fatalf("expected YARN_REGISTRY alias to be synthesized, got %q", got["YARN_REGISTRY"])
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

func TestExpandBuildEnvsForSlugBuildBackfillsBlankLegacyAliases(t *testing.T) {
	envs := map[string]string{
		"BUILD_GOPROXY":                 "https://goproxy.cn",
		"BUILD_GOPRIVATE":               "github.com/acme/*",
		"BUILD_GO_INSTALL_PACKAGE_SPEC": "./cmd/demo",
		"GOPROXY":                       "",
		"GOPRIVATE":                     "",
		"GO_INSTALL_PACKAGE_SPEC":       "",
	}

	got := expandBuildEnvsForSlugBuild(envs)

	if got["GOPROXY"] != envs["BUILD_GOPROXY"] {
		t.Fatalf("expected blank GOPROXY alias to be backfilled, got %q", got["GOPROXY"])
	}
	if got["GOPRIVATE"] != envs["BUILD_GOPRIVATE"] {
		t.Fatalf("expected blank GOPRIVATE alias to be backfilled, got %q", got["GOPRIVATE"])
	}
	if got["GO_INSTALL_PACKAGE_SPEC"] != envs["BUILD_GO_INSTALL_PACKAGE_SPEC"] {
		t.Fatalf("expected blank GO_INSTALL_PACKAGE_SPEC alias to be backfilled, got %q", got["GO_INSTALL_PACKAGE_SPEC"])
	}
}

func TestExpandBuildEnvsForSlugBuildAddsNodeCompatibilityAliases(t *testing.T) {
	envs := map[string]string{
		"BUILD_PACKAGE_TOOL": "pnpm",
		"BUILD_OUTPUT_DIR":   "build",
		"BUILD_BUILD_CMD":    "pnpm run build",
	}

	got := expandBuildEnvsForSlugBuild(envs)

	if got["PACKAGE_TOOL"] != "pnpm" {
		t.Fatalf("expected PACKAGE_TOOL alias to be synthesized, got %q", got["PACKAGE_TOOL"])
	}
	if got["DIST_DIR"] != "build" {
		t.Fatalf("expected DIST_DIR alias to be synthesized from BUILD_OUTPUT_DIR, got %q", got["DIST_DIR"])
	}
	if got["NODE_BUILD_CMD"] != "pnpm run build" {
		t.Fatalf("expected NODE_BUILD_CMD alias to be synthesized from BUILD_BUILD_CMD, got %q", got["NODE_BUILD_CMD"])
	}
}

func TestExpandBuildEnvsForSlugBuildPrefersExplicitNodeCompatAliases(t *testing.T) {
	envs := map[string]string{
		"BUILD_DIST_DIR":      "public",
		"BUILD_OUTPUT_DIR":    "build",
		"BUILD_NODE_BUILD_CMD": "npm run build",
		"BUILD_BUILD_CMD":     "pnpm run build",
	}

	got := expandBuildEnvsForSlugBuild(envs)

	if got["DIST_DIR"] != "public" {
		t.Fatalf("expected BUILD_DIST_DIR to win for DIST_DIR, got %q", got["DIST_DIR"])
	}
	if got["NODE_BUILD_CMD"] != "npm run build" {
		t.Fatalf("expected BUILD_NODE_BUILD_CMD to win for NODE_BUILD_CMD, got %q", got["NODE_BUILD_CMD"])
	}
}
