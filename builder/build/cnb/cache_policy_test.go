package cnb

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/goodrain/rainbond/builder/build"
	"github.com/goodrain/rainbond/builder/parser/code"
	"github.com/goodrain/rainbond/event"
)

// capability_id: rainbond.cnb.npm-cache-policy
func TestCNBNPMCachePolicy(t *testing.T) {
	cases := []struct {
		name          string
		files         map[string]string
		dirs          []string
		envs          map[string]string
		lang          code.Lang
		registryCache bool
		reuseImage    bool
	}{
		{name: "npm install skips remote cache", reuseImage: true},
		{name: "npm declaration without lock", files: map[string]string{"package.json": `{"packageManager":"npm@10.8.2"}`}, reuseImage: true},
		{name: "unreadable package metadata preserves cache", files: map[string]string{"package.json": `{`}, registryCache: true, reuseImage: true},
		{name: "lock file retains cache", files: map[string]string{"package-lock.json": `{}`}, registryCache: true, reuseImage: true},
		{name: "shrinkwrap conservatively retains cache", files: map[string]string{"npm-shrinkwrap.json": `{}`}, registryCache: true, reuseImage: true},
		{name: "vendored modules retain cache", dirs: []string{"node_modules"}, registryCache: true, reuseImage: true},
		{name: "yarn lock retains cache", files: map[string]string{"yarn.lock": "lock"}, registryCache: true, reuseImage: true},
		{name: "pnpm lock retains cache", files: map[string]string{"pnpm-lock.yaml": "lock"}, registryCache: true, reuseImage: true},
		{name: "yarn declaration retains cache", files: map[string]string{"package.json": `{"packageManager":"yarn@4.0.0"}`}, registryCache: true, reuseImage: true},
		{name: "pnpm declaration retains cache", files: map[string]string{"package.json": `{"packageManager":"pnpm@9.0.0"}`}, registryCache: true, reuseImage: true},
		{name: "npm declaration does not override yarn lock", files: map[string]string{"package.json": `{"packageManager":"npm@10.0.0"}`, "yarn.lock": "lock"}, registryCache: true, reuseImage: true},
		{name: "non Node language retains cache", lang: code.JavaMaven, registryCache: true, reuseImage: true},
		{name: "pure static retains cache", lang: code.Static, registryCache: true, reuseImage: true},
		{name: "explicit no cache", envs: map[string]string{"NO_CACHE": "true"}},
		{name: "build no cache alias", envs: map[string]string{"BUILD_NO_CACHE": "ON"}},
		{name: "alias takes precedence over false", envs: map[string]string{"NO_CACHE": "false", "BUILD_NO_CACHE": "true"}},
		{name: "explicit disable with lock", files: map[string]string{"package-lock.json": `{}`}, envs: map[string]string{"NO_CACHE": "true"}},
		{name: "subdirectory uses its own lock", files: map[string]string{"frontend/package.json": `{}`, "frontend/package-lock.json": `{}`}, envs: map[string]string{"BP_NODE_PROJECT_PATH": "frontend"}, registryCache: true, reuseImage: true},
		{name: "root lock does not cover subdirectory", files: map[string]string{"package-lock.json": `{}`, "frontend/package.json": `{}`}, envs: map[string]string{"BP_NODE_PROJECT_PATH": "frontend"}, reuseImage: true},
		{name: "unknown subdirectory preserves cache", envs: map[string]string{"BP_NODE_PROJECT_PATH": "missing"}, registryCache: true, reuseImage: true},
		{name: "escaped subdirectory preserves cache", envs: map[string]string{"BP_NODE_PROJECT_PATH": "../outside"}, registryCache: true, reuseImage: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir := newNodeDir(t)
			for name, content := range tc.files {
				filename := filepath.Join(dir, name)
				if err := os.MkdirAll(filepath.Dir(filename), 0755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filename, []byte(content), 0644); err != nil {
					t.Fatal(err)
				}
			}
			for _, name := range tc.dirs {
				if err := os.MkdirAll(filepath.Join(dir, name), 0755); err != nil {
					t.Fatal(err)
				}
			}
			lang := tc.lang
			if lang == "" {
				lang = code.Nodejs
			}
			args := (&Builder{}).buildCreatorArgs(&build.Request{SourceDir: dir, Lang: lang, BuildEnvs: tc.envs}, "registry.test/app:v1", "run:v1")
			for flag, want := range map[string]bool{
				"-cache-image=":    tc.registryCache,
				"-previous-image=": tc.reuseImage,
				"-parallel":        tc.registryCache,
				"-skip-restore":    !tc.reuseImage,
			} {
				found := false
				for _, arg := range args {
					if strings.HasPrefix(arg, flag) {
						found = true
					}
				}
				if found != want {
					t.Errorf("flag %s present = %v, want %v; args: %v", flag, found, want, args)
				}
			}
		})
	}
}

// capability_id: rainbond.cnb.npm-cache-policy
func TestCNBCachePolicyExplainsAutomaticBypass(t *testing.T) {
	logs := make(chan []byte, 10)
	re := &build.Request{SourceDir: newNodeDir(t), Lang: code.Nodejs, Logger: event.NewLogger("test", logs)}
	(&Builder{}).buildCreatorArgs(re, "registry.test/app:v1", "run:v1")
	select {
	case data := <-logs:
		var entry map[string]string
		if err := json.Unmarshal(data, &entry); err != nil {
			t.Fatal(err)
		}
		if entry["step"] != "build-cache" || !strings.Contains(entry["message"], "npm install") || !strings.Contains(entry["message"], "image layer reuse enabled") {
			t.Fatalf("expected a user-visible cache strategy explanation, got %v", entry)
		}
	default:
		t.Fatal("cache strategy was not logged")
	}
}

// capability_id: rainbond.cnb.npm-cache-policy
func TestCNBCachePolicyPreservesCacheForUnknownSources(t *testing.T) {
	for _, source := range []string{"", t.TempDir(), filepath.Join(t.TempDir(), "missing")} {
		policy := resolveCNBCachePolicy(&build.Request{SourceDir: source, Lang: code.Nodejs})
		if !policy.registryCache || !policy.reuseImage {
			t.Fatalf("unknown source %q lost its existing cache strategy: %+v", source, policy)
		}
	}
}
