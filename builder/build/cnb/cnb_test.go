package cnb

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/eapache/channels"
	"github.com/goodrain/rainbond/builder/build"
	"github.com/goodrain/rainbond/builder/parser/code"
	"github.com/goodrain/rainbond/event"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"
)

// --- helpers ---

func newNodeDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "package.json"), []byte(`{"name":"test"}`), 0644)
	return dir
}

// --- config.go ---

func TestGetCNBBuilderImage(t *testing.T) {
	t.Run("default", func(t *testing.T) {
		os.Unsetenv("CNB_BUILDER_IMAGE")
		if got := GetCNBBuilderImage(); got != DefaultCNBBuilder {
			t.Errorf("got %q; want %q", got, DefaultCNBBuilder)
		}
	})
	t.Run("custom", func(t *testing.T) {
		os.Setenv("CNB_BUILDER_IMAGE", "custom:latest")
		defer os.Unsetenv("CNB_BUILDER_IMAGE")
		if got := GetCNBBuilderImage(); got != "custom:latest" {
			t.Errorf("got %q; want %q", got, "custom:latest")
		}
	})
}

func TestGetCNBRunImage(t *testing.T) {
	os.Unsetenv("CNB_RUN_IMAGE")
	if got := GetCNBRunImage(); got != DefaultCNBRunImage {
		t.Errorf("got %q; want %q", got, DefaultCNBRunImage)
	}
}

// --- order.go ---

func TestWriteCustomOrder(t *testing.T) {
	b := &Builder{}

	t.Run("writes order.toml with version", func(t *testing.T) {
		dir := t.TempDir()
		re := &build.Request{SourceDir: dir}
		bps := []orderBuildpack{{ID: "paketo-buildpacks/nginx", Version: "1.0.12"}}
		flag := b.writeCustomOrder(re, bps, "test")
		if flag != "-order=/workspace/.cnb-order.toml" {
			t.Errorf("got flag %q", flag)
		}
		content, _ := os.ReadFile(filepath.Join(dir, ".cnb-order.toml"))
		if !strings.Contains(string(content), `id = "paketo-buildpacks/nginx"`) {
			t.Error("expected nginx buildpack ID")
		}
		if !strings.Contains(string(content), `version = "1.0.12"`) {
			t.Error("expected version")
		}
	})

	t.Run("optional buildpack", func(t *testing.T) {
		dir := t.TempDir()
		re := &build.Request{SourceDir: dir}
		bps := []orderBuildpack{{ID: "bp/test", Optional: true}}
		b.writeCustomOrder(re, bps, "test")
		content, _ := os.ReadFile(filepath.Join(dir, ".cnb-order.toml"))
		if !strings.Contains(string(content), "optional = true") {
			t.Error("expected optional = true")
		}
	})

	t.Run("no version omits version line", func(t *testing.T) {
		dir := t.TempDir()
		re := &build.Request{SourceDir: dir}
		bps := []orderBuildpack{{ID: "bp/test"}}
		b.writeCustomOrder(re, bps, "test")
		content, _ := os.ReadFile(filepath.Join(dir, ".cnb-order.toml"))
		if strings.Contains(string(content), "version") {
			t.Error("expected no version line")
		}
	})
}

func TestStaticBuildpacks(t *testing.T) {
	s := &staticConfig{}
	dir := t.TempDir() // no package.json = pure static
	re := &build.Request{SourceDir: dir, BuildEnvs: map[string]string{}}
	bps := s.CustomOrder(re)
	if len(bps) != 1 || bps[0].ID != "paketo-buildpacks/nginx" {
		t.Errorf("expected single nginx buildpack, got %+v", bps)
	}
}

// --- mirror.go ---

func TestInjectMirrorConfig(t *testing.T) {
	n := &nodejsConfig{}

	t.Run("skip for pure static", func(t *testing.T) {
		dir := t.TempDir()
		re := &build.Request{SourceDir: dir, BuildEnvs: map[string]string{}}
		n.InjectMirrorConfig(re)
		if _, err := os.Stat(filepath.Join(dir, ".npmrc")); !os.IsNotExist(err) {
			t.Error("expected no .npmrc for pure static")
		}
	})

	t.Run("project source with existing config", func(t *testing.T) {
		dir := newNodeDir(t)
		os.WriteFile(filepath.Join(dir, ".npmrc"), []byte("custom"), 0644)
		re := &build.Request{SourceDir: dir, BuildEnvs: map[string]string{"CNB_MIRROR_SOURCE": "project"}}
		n.InjectMirrorConfig(re)
		content, _ := os.ReadFile(filepath.Join(dir, ".npmrc"))
		if string(content) != "custom" {
			t.Error("project .npmrc should be preserved")
		}
	})

	t.Run("project source without config falls through", func(t *testing.T) {
		dir := newNodeDir(t)
		re := &build.Request{SourceDir: dir, BuildEnvs: map[string]string{"CNB_MIRROR_SOURCE": "project"}}
		n.InjectMirrorConfig(re)
		if _, err := os.Stat(filepath.Join(dir, ".npmrc")); os.IsNotExist(err) {
			t.Error("expected .npmrc fallback")
		}
	})

	t.Run("creates both npmrc and yarnrc", func(t *testing.T) {
		dir := newNodeDir(t)
		re := &build.Request{SourceDir: dir, BuildEnvs: map[string]string{}}
		n.InjectMirrorConfig(re)
		for _, f := range []string{".npmrc", ".yarnrc"} {
			if _, err := os.Stat(filepath.Join(dir, f)); os.IsNotExist(err) {
				t.Errorf("expected %s to be created", f)
			}
		}
	})
}

func TestInjectConfigFile(t *testing.T) {
	t.Run("existing file not overwritten", func(t *testing.T) {
		dir := t.TempDir()
		os.WriteFile(filepath.Join(dir, ".npmrc"), []byte("existing"), 0644)
		re := &build.Request{SourceDir: dir, BuildEnvs: map[string]string{}}
		injectConfigFile(re, ".npmrc", "CNB_MIRROR_NPMRC")
		content, _ := os.ReadFile(filepath.Join(dir, ".npmrc"))
		if string(content) != "existing" {
			t.Error("should not overwrite")
		}
	})

	t.Run("user provided content", func(t *testing.T) {
		dir := t.TempDir()
		re := &build.Request{SourceDir: dir, BuildEnvs: map[string]string{"CNB_MIRROR_NPMRC": "custom"}}
		injectConfigFile(re, ".npmrc", "CNB_MIRROR_NPMRC")
		content, _ := os.ReadFile(filepath.Join(dir, ".npmrc"))
		if string(content) != "custom" {
			t.Errorf("got %q; want custom", string(content))
		}
	})

	t.Run("china mirror disabled", func(t *testing.T) {
		dir := t.TempDir()
		os.Setenv("ENABLE_CHINA_MIRROR", "false")
		defer os.Unsetenv("ENABLE_CHINA_MIRROR")
		re := &build.Request{SourceDir: dir, BuildEnvs: map[string]string{}}
		injectConfigFile(re, ".npmrc", "CNB_MIRROR_NPMRC")
		if _, err := os.Stat(filepath.Join(dir, ".npmrc")); !os.IsNotExist(err) {
			t.Error("expected no .npmrc when mirror disabled")
		}
	})

	t.Run("default npmrc", func(t *testing.T) {
		dir := t.TempDir()
		os.Setenv("ENABLE_CHINA_MIRROR", "true")
		defer os.Unsetenv("ENABLE_CHINA_MIRROR")
		re := &build.Request{SourceDir: dir, BuildEnvs: map[string]string{}}
		injectConfigFile(re, ".npmrc", "CNB_MIRROR_NPMRC")
		content, _ := os.ReadFile(filepath.Join(dir, ".npmrc"))
		if string(content) != DefaultNpmrcContent {
			t.Errorf("got %q; want default", string(content))
		}
	})

	t.Run("default yarnrc", func(t *testing.T) {
		dir := t.TempDir()
		os.Setenv("ENABLE_CHINA_MIRROR", "true")
		defer os.Unsetenv("ENABLE_CHINA_MIRROR")
		re := &build.Request{SourceDir: dir, BuildEnvs: map[string]string{}}
		injectConfigFile(re, ".yarnrc", "CNB_MIRROR_YARNRC")
		content, _ := os.ReadFile(filepath.Join(dir, ".yarnrc"))
		if string(content) != DefaultYarnrcContent {
			t.Errorf("got %q; want default", string(content))
		}
	})

	t.Run("unknown file type", func(t *testing.T) {
		dir := t.TempDir()
		re := &build.Request{SourceDir: dir, BuildEnvs: map[string]string{}}
		if err := injectConfigFile(re, ".unknown", "K"); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})
}

// --- platform.go ---

func TestBpEnvToAnnotationKey(t *testing.T) {
	tests := []struct{ input, want string }{
		{"BP_NODE_VERSION", "cnb-bp-node-version"},
		{"BP_WEB_SERVER", "cnb-bp-web-server"},
		{"BP_", "cnb-bp-"},
	}
	for _, tt := range tests {
		if got := bpEnvToAnnotationKey(tt.input); got != tt.want {
			t.Errorf("bpEnvToAnnotationKey(%q) = %q; want %q", tt.input, got, tt.want)
		}
	}
}

func TestAnnotationKeyToBPEnv(t *testing.T) {
	tests := []struct{ input, want string }{
		{"cnb-bp-node-version", "BP_NODE_VERSION"},
		{"cnb-bp-web-server", "BP_WEB_SERVER"},
		{"cnb-node-env", "NODE_ENV"},
	}
	for _, tt := range tests {
		if got := annotationKeyToBPEnv(tt.input); got != tt.want {
			t.Errorf("annotationKeyToBPEnv(%q) = %q; want %q", tt.input, got, tt.want)
		}
	}
}

func TestBuildPlatformAnnotations(t *testing.T) {
	nodeDir := newNodeDir(t)

	t.Run("node.js defaults", func(t *testing.T) {
		ann := (&Builder{}).buildPlatformAnnotations(&build.Request{SourceDir: nodeDir, BuildEnvs: map[string]string{}})
		if ann["cnb-node-env"] != "production" {
			t.Errorf("cnb-node-env = %q; want production", ann["cnb-node-env"])
		}
	})

	t.Run("custom NODE_ENV", func(t *testing.T) {
		ann := (&Builder{}).buildPlatformAnnotations(&build.Request{SourceDir: nodeDir, BuildEnvs: map[string]string{"CNB_NODE_ENV": "development"}})
		if ann["cnb-node-env"] != "development" {
			t.Errorf("got %q", ann["cnb-node-env"])
		}
	})

	t.Run("node version", func(t *testing.T) {
		ann := (&Builder{}).buildPlatformAnnotations(&build.Request{SourceDir: nodeDir, BuildEnvs: map[string]string{"CNB_NODE_VERSION": "20.10.0"}})
		if ann["cnb-bp-node-version"] != "20.10.0" {
			t.Errorf("got %q", ann["cnb-bp-node-version"])
		}
	})

	t.Run("RUNTIMES fallback", func(t *testing.T) {
		ann := (&Builder{}).buildPlatformAnnotations(&build.Request{SourceDir: nodeDir, BuildEnvs: map[string]string{"RUNTIMES": "18.17.0"}})
		if ann["cnb-bp-node-version"] != "18.17.0" {
			t.Errorf("got %q", ann["cnb-bp-node-version"])
		}
	})

	t.Run("static build", func(t *testing.T) {
		ann := (&Builder{}).buildPlatformAnnotations(&build.Request{SourceDir: nodeDir, BuildEnvs: map[string]string{"CNB_OUTPUT_DIR": "dist"}})
		if ann["cnb-bp-web-server"] != "nginx" {
			t.Error("expected nginx")
		}
		if ann["cnb-bp-web-server-root"] != "dist" {
			t.Errorf("got root %q", ann["cnb-bp-web-server-root"])
		}
	})

	t.Run("server framework skips nginx", func(t *testing.T) {
		ann := (&Builder{}).buildPlatformAnnotations(&build.Request{SourceDir: nodeDir, BuildEnvs: map[string]string{
			"CNB_FRAMEWORK":  "nestjs",
			"CNB_OUTPUT_DIR": "dist",
		}})
		if _, ok := ann["cnb-bp-web-server"]; ok {
			t.Error("server framework should not have nginx")
		}
	})

	t.Run("nextjs SSR skips nginx", func(t *testing.T) {
		ann := (&Builder{}).buildPlatformAnnotations(&build.Request{SourceDir: nodeDir, BuildEnvs: map[string]string{
			"CNB_FRAMEWORK": "nextjs",
		}})
		if _, ok := ann["cnb-bp-web-server"]; ok {
			t.Error("nextjs SSR without CNB_OUTPUT_DIR should not have nginx")
		}
	})

	t.Run("nextjs static export gets nginx", func(t *testing.T) {
		ann := (&Builder{}).buildPlatformAnnotations(&build.Request{SourceDir: nodeDir, BuildEnvs: map[string]string{
			"CNB_FRAMEWORK":  "nextjs-static",
			"CNB_OUTPUT_DIR": "out",
		}})
		if ann["cnb-bp-web-server"] != "nginx" {
			t.Error("nextjs-static should have nginx")
		}
		if ann["cnb-bp-web-server-root"] != "out" {
			t.Errorf("expected output dir 'out', got %q", ann["cnb-bp-web-server-root"])
		}
	})

	t.Run("nuxt static export gets nginx", func(t *testing.T) {
		ann := (&Builder{}).buildPlatformAnnotations(&build.Request{SourceDir: nodeDir, BuildEnvs: map[string]string{
			"CNB_FRAMEWORK":  "nuxt-static",
			"CNB_OUTPUT_DIR": "dist",
		}})
		if ann["cnb-bp-web-server"] != "nginx" {
			t.Error("nuxt-static should have nginx")
		}
	})

	t.Run("static framework gets nginx", func(t *testing.T) {
		ann := (&Builder{}).buildPlatformAnnotations(&build.Request{SourceDir: nodeDir, BuildEnvs: map[string]string{
			"CNB_FRAMEWORK": "react",
		}})
		if ann["cnb-bp-web-server"] != "nginx" {
			t.Error("static framework should have nginx")
		}
		if ann["cnb-bp-web-server-root"] != "dist" {
			t.Errorf("default output dir should be dist, got %q", ann["cnb-bp-web-server-root"])
		}
	})

	t.Run("static framework with custom output dir", func(t *testing.T) {
		ann := (&Builder{}).buildPlatformAnnotations(&build.Request{SourceDir: nodeDir, BuildEnvs: map[string]string{
			"CNB_FRAMEWORK":  "vue",
			"CNB_OUTPUT_DIR": "build",
		}})
		if ann["cnb-bp-web-server-root"] != "build" {
			t.Errorf("got %q; want build", ann["cnb-bp-web-server-root"])
		}
	})

	t.Run("build script", func(t *testing.T) {
		ann := (&Builder{}).buildPlatformAnnotations(&build.Request{SourceDir: nodeDir, BuildEnvs: map[string]string{"CNB_BUILD_SCRIPT": "build:prod"}})
		if ann["cnb-bp-node-run-scripts"] != "build:prod" {
			t.Errorf("got %q", ann["cnb-bp-node-run-scripts"])
		}
	})

	t.Run("BP_ passthrough", func(t *testing.T) {
		ann := (&Builder{}).buildPlatformAnnotations(&build.Request{SourceDir: nodeDir, BuildEnvs: map[string]string{"BP_CUSTOM_VAR": "val"}})
		if ann["cnb-bp-custom-var"] != "val" {
			t.Error("BP_ passthrough failed")
		}
	})

	t.Run("CNB_START_SCRIPT to BP_NPM_START_SCRIPT", func(t *testing.T) {
		ann := (&Builder{}).buildPlatformAnnotations(&build.Request{SourceDir: nodeDir, BuildEnvs: map[string]string{"CNB_START_SCRIPT": "start:prod"}})
		if ann["cnb-bp-npm-start-script"] != "start:prod" {
			t.Errorf("expected start:prod, got %q", ann["cnb-bp-npm-start-script"])
		}
	})

	t.Run("CNB_START_SCRIPT with pnpm", func(t *testing.T) {
		ann := (&Builder{}).buildPlatformAnnotations(&build.Request{SourceDir: nodeDir, BuildEnvs: map[string]string{
			"CNB_START_SCRIPT":   "serve",
			"CNB_PACKAGE_TOOL": "pnpm",
		}})
		if ann["cnb-bp-npm-start-script"] != "serve" {
			t.Errorf("expected serve, got %q", ann["cnb-bp-npm-start-script"])
		}
	})

	t.Run("BP_NPM_START_SCRIPT passthrough", func(t *testing.T) {
		ann := (&Builder{}).buildPlatformAnnotations(&build.Request{SourceDir: nodeDir, BuildEnvs: map[string]string{"BP_NPM_START_SCRIPT": "start:prod"}})
		if ann["cnb-bp-npm-start-script"] != "start:prod" {
			t.Errorf("expected start:prod, got %q", ann["cnb-bp-npm-start-script"])
		}
	})

	t.Run("pure static project", func(t *testing.T) {
		dir := t.TempDir()
		ann := (&Builder{}).buildPlatformAnnotations(&build.Request{SourceDir: dir, BuildEnvs: map[string]string{}})
		if ann["cnb-bp-web-server"] != "nginx" {
			t.Error("pure static should have nginx")
		}
		if ann["cnb-bp-web-server-root"] != "." {
			t.Errorf("got %q; want '.'", ann["cnb-bp-web-server-root"])
		}
	})

	t.Run("pure static with custom output dir", func(t *testing.T) {
		dir := t.TempDir()
		ann := (&Builder{}).buildPlatformAnnotations(&build.Request{SourceDir: dir, BuildEnvs: map[string]string{"CNB_OUTPUT_DIR": "public"}})
		if ann["cnb-bp-web-server-root"] != "public" {
			t.Errorf("got %q; want public", ann["cnb-bp-web-server-root"])
		}
	})
}

func TestCreatePlatformVolume(t *testing.T) {
	b := &Builder{}
	dir := newNodeDir(t)
	re := &build.Request{SourceDir: dir, BuildEnvs: map[string]string{"CNB_NODE_VERSION": "20"}}
	annotations := b.buildPlatformAnnotations(re)
	vol, mount := b.createPlatformVolume(annotations)
	if vol == nil || mount == nil {
		t.Fatal("expected volume and mount")
	}
	if vol.Name != "platform" || mount.MountPath != "/platform" {
		t.Errorf("vol=%q mount=%q", vol.Name, mount.MountPath)
	}
	if len(vol.DownwardAPI.Items) == 0 {
		t.Error("expected DownwardAPI items")
	}
}

// --- job.go ---

func TestBuildEnvVars(t *testing.T) {
	b := &Builder{}

	t.Run("basic envs", func(t *testing.T) {
		envs := b.buildEnvVars(&build.Request{BuildEnvs: map[string]string{}})
		m := make(map[string]string)
		for _, e := range envs {
			m[e.Name] = e.Value
		}
		if m["CNB_PLATFORM_API"] != "0.13" {
			t.Error("expected CNB_PLATFORM_API=0.13")
		}
		if m["DOCKER_CONFIG"] != "/home/cnb/.docker" {
			t.Error("expected DOCKER_CONFIG=/home/cnb/.docker")
		}
	})

	t.Run("proxy settings", func(t *testing.T) {
		os.Setenv("HTTP_PROXY", "http://p:8080")
		os.Setenv("HTTPS_PROXY", "https://p:8443")
		os.Setenv("NO_PROXY", "localhost")
		defer func() {
			os.Unsetenv("HTTP_PROXY")
			os.Unsetenv("HTTPS_PROXY")
			os.Unsetenv("NO_PROXY")
		}()
		envs := b.buildEnvVars(&build.Request{BuildEnvs: map[string]string{}})
		m := make(map[string]string)
		for _, e := range envs {
			m[e.Name] = e.Value
		}
		if m["HTTP_PROXY"] != "http://p:8080" {
			t.Error("HTTP_PROXY not set")
		}
	})
}

func TestBuildCreatorArgs(t *testing.T) {
	b := &Builder{}

	t.Run("basic args", func(t *testing.T) {
		dir := newNodeDir(t)
		re := &build.Request{SourceDir: dir, BuildEnvs: map[string]string{}}
		args := b.buildCreatorArgs(re, "img:v1", "run:v1")
		for _, r := range []string{"-app=/workspace", "-layers=/layers", "-platform=/platform"} {
			found := false
			for _, a := range args {
				if a == r {
					found = true
				}
			}
			if !found {
				t.Errorf("missing arg %s", r)
			}
		}
		if args[len(args)-1] != "img:v1" {
			t.Errorf("last arg = %q", args[len(args)-1])
		}
	})

	t.Run("pure static gets custom order", func(t *testing.T) {
		dir := t.TempDir()
		args := b.buildCreatorArgs(&build.Request{SourceDir: dir, BuildEnvs: map[string]string{}}, "img:v1", "run:v1")
		hasOrder := false
		for _, a := range args {
			if strings.HasPrefix(a, "-order=") {
				hasOrder = true
			}
		}
		if !hasOrder {
			t.Error("expected -order flag for pure static")
		}
	})

	t.Run("node project uses default order", func(t *testing.T) {
		dir := newNodeDir(t)
		args := b.buildCreatorArgs(&build.Request{SourceDir: dir, BuildEnvs: map[string]string{}}, "img:v1", "run:v1")
		for _, a := range args {
			if strings.HasPrefix(a, "-order=") {
				t.Error("node project should use default builder order, not custom order")
			}
		}
	})
}

func TestCreateVolumeAndMount(t *testing.T) {
	b := &Builder{}
	re := &build.Request{SourceDir: "/tmp/src", CacheDir: "/tmp/cache"}
	vols, mounts := b.createVolumeAndMount(re, "secret-name")
	if len(vols) != 5 {
		t.Errorf("expected 5 volumes, got %d", len(vols))
	}
	if len(mounts) != 5 {
		t.Errorf("expected 5 mounts, got %d", len(mounts))
	}
	// Check docker-config uses secret
	found := false
	for _, v := range vols {
		if v.Name == "docker-config" && v.Secret != nil && v.Secret.SecretName == "secret-name" {
			found = true
		}
	}
	if !found {
		t.Error("expected docker-config volume with secret")
	}
}

// --- build.go ---

func TestSetSourceDirPermissions(t *testing.T) {
	b := &Builder{}

	t.Run("removes .git and sets permissions recursively", func(t *testing.T) {
		dir := t.TempDir()
		gitDir := filepath.Join(dir, ".git")
		os.Mkdir(gitDir, 0755)
		os.WriteFile(filepath.Join(gitDir, "config"), []byte("test"), 0644)
		subDir := filepath.Join(dir, "src")
		os.Mkdir(subDir, 0755)
		os.WriteFile(filepath.Join(subDir, "main.go"), []byte("package main"), 0644)
		b.setSourceDirPermissions(&build.Request{SourceDir: dir})
		if _, err := os.Stat(gitDir); !os.IsNotExist(err) {
			t.Error(".git should be removed")
		}
		info, _ := os.Stat(dir)
		if info.Mode().Perm() != 0777 {
			t.Errorf("dir permissions = %o; want 0777", info.Mode().Perm())
		}
		subInfo, _ := os.Stat(subDir)
		if subInfo.Mode().Perm() != 0777 {
			t.Errorf("subdir permissions = %o; want 0777", subInfo.Mode().Perm())
		}
		fileInfo, _ := os.Stat(filepath.Join(subDir, "main.go"))
		if fileInfo.Mode().Perm() != 0777 {
			t.Errorf("file permissions = %o; want 0777", fileInfo.Mode().Perm())
		}
	})

	t.Run("no .git directory", func(t *testing.T) {
		dir := t.TempDir()
		if err := b.setSourceDirPermissions(&build.Request{SourceDir: dir}); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})
}

func TestNewBuilder(t *testing.T) {
	b, err := NewBuilder()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if b == nil {
		t.Error("expected non-nil builder")
	}
}

func TestValidateProjectFiles(t *testing.T) {
	b := &Builder{}
	logger := event.NewLogger("test", nil)

	t.Run("empty lock file returns error", func(t *testing.T) {
		dir := t.TempDir()
		os.WriteFile(filepath.Join(dir, "package-lock.json"), []byte{}, 0644)
		re := &build.Request{SourceDir: dir, Logger: logger}
		if err := b.validateProjectFiles(re); err == nil {
			t.Error("expected error for empty lock file")
		}
	})

	t.Run("valid lock file passes", func(t *testing.T) {
		dir := t.TempDir()
		os.WriteFile(filepath.Join(dir, "package-lock.json"), []byte(`{"lockfileVersion":3}`), 0644)
		re := &build.Request{SourceDir: dir, Logger: logger}
		if err := b.validateProjectFiles(re); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("no lock file passes", func(t *testing.T) {
		dir := t.TempDir()
		re := &build.Request{SourceDir: dir, Logger: logger}
		if err := b.validateProjectFiles(re); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})
}

// noopLogger and related types removed - using event.NewLogger instead

// --- waitingComplete (job.go) ---

func TestWaitingComplete(t *testing.T) {
	b := &Builder{}
	logger := event.GetTestLogger()

	t.Run("complete then logcomplete", func(t *testing.T) {
		reChan := channels.NewRingChannel(10)
		re := &build.Request{Logger: logger}
		go func() {
			reChan.In() <- "complete"
			reChan.In() <- "logcomplete"
		}()
		if err := b.waitingComplete(re, reChan); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("logcomplete then complete", func(t *testing.T) {
		reChan := channels.NewRingChannel(10)
		re := &build.Request{Logger: logger}
		go func() {
			reChan.In() <- "logcomplete"
			reChan.In() <- "complete"
		}()
		if err := b.waitingComplete(re, reChan); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("failed then logcomplete", func(t *testing.T) {
		reChan := channels.NewRingChannel(10)
		re := &build.Request{Logger: logger}
		go func() {
			reChan.In() <- "failed"
			reChan.In() <- "logcomplete"
		}()
		err := b.waitingComplete(re, reChan)
		if err == nil {
			t.Error("expected error for failed job")
		}
	})

	t.Run("logcomplete then failed", func(t *testing.T) {
		reChan := channels.NewRingChannel(10)
		re := &build.Request{Logger: logger}
		go func() {
			reChan.In() <- "logcomplete"
			reChan.In() <- "failed"
		}()
		err := b.waitingComplete(re, reChan)
		if err == nil {
			t.Error("expected error for failed job")
		}
	})

	t.Run("cancel then logcomplete", func(t *testing.T) {
		reChan := channels.NewRingChannel(10)
		re := &build.Request{Logger: logger}
		go func() {
			reChan.In() <- "cancel"
			reChan.In() <- "logcomplete"
		}()
		err := b.waitingComplete(re, reChan)
		if err == nil {
			t.Error("expected error for canceled job")
		}
		if !strings.Contains(err.Error(), "canceled") {
			t.Errorf("expected 'canceled' in error, got %q", err.Error())
		}
	})

	t.Run("non-string status ignored", func(t *testing.T) {
		reChan := channels.NewRingChannel(10)
		re := &build.Request{Logger: logger}
		go func() {
			reChan.In() <- 12345
			reChan.In() <- "complete"
			reChan.In() <- "logcomplete"
		}()
		if err := b.waitingComplete(re, reChan); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})
}

// --- createPlatformVolume edge cases ---

func TestCreatePlatformVolumeEmpty(t *testing.T) {
	b := &Builder{}
	vol, mount := b.createPlatformVolume(map[string]string{})
	if vol != nil || mount != nil {
		t.Error("expected nil for empty annotations")
	}
}

func TestCreatePlatformVolumeNonCNBKeys(t *testing.T) {
	b := &Builder{}
	vol, mount := b.createPlatformVolume(map[string]string{"other-key": "val"})
	if vol != nil || mount != nil {
		t.Error("expected nil when no cnb- prefixed keys")
	}
}

// --- writeCustomOrder error path ---

func TestWriteCustomOrderFailure(t *testing.T) {
	b := &Builder{}
	re := &build.Request{SourceDir: "/nonexistent/path/that/does/not/exist"}
	bps := []orderBuildpack{{ID: "bp/test"}}
	flag := b.writeCustomOrder(re, bps, "test")
	if flag != "" {
		t.Errorf("expected empty flag on write failure, got %q", flag)
	}
}

// --- injectMirrorConfig error propagation ---

func TestInjectMirrorConfigWriteError(t *testing.T) {
	n := &nodejsConfig{}
	dir := newNodeDir(t)
	// Make dir read-only so WriteFile fails
	os.Chmod(dir, 0555)
	defer os.Chmod(dir, 0755)
	re := &build.Request{SourceDir: dir, BuildEnvs: map[string]string{
		"CNB_MIRROR_NPMRC": "custom-content",
	}}
	err := n.InjectMirrorConfig(re)
	if err == nil {
		t.Error("expected error when directory is read-only")
	}
}

// --- setSourceDirPermissions with nonexistent dir ---

func TestSetSourceDirPermissionsNonexistent(t *testing.T) {
	b := &Builder{}
	err := b.setSourceDirPermissions(&build.Request{SourceDir: "/nonexistent/path"})
	if err == nil {
		t.Error("expected error for nonexistent directory")
	}
}

// --- dependency mirror only set when env var is explicitly configured ---

func TestBuildPlatformAnnotationsMirrorDefault(t *testing.T) {
	os.Unsetenv("BP_DEPENDENCY_MIRROR")
	dir := newNodeDir(t)
	ann := (&Builder{}).buildPlatformAnnotations(&build.Request{SourceDir: dir, BuildEnvs: map[string]string{}})
	if _, ok := ann["cnb-bp-dependency-mirror"]; ok {
		t.Error("expected no dependency mirror when BP_DEPENDENCY_MIRROR is not set")
	}
}

func TestBuildPlatformAnnotationsMirrorExplicit(t *testing.T) {
	os.Setenv("BP_DEPENDENCY_MIRROR", "https://example.com/mirror")
	defer os.Unsetenv("BP_DEPENDENCY_MIRROR")
	dir := newNodeDir(t)
	ann := (&Builder{}).buildPlatformAnnotations(&build.Request{SourceDir: dir, BuildEnvs: map[string]string{}})
	if ann["cnb-bp-dependency-mirror"] != "https://example.com/mirror" {
		t.Errorf("expected dependency mirror from env, got %q", ann["cnb-bp-dependency-mirror"])
	}
}

// --- BP_ passthrough does not override explicit keys ---

func TestBuildPlatformAnnotationsBPNoOverride(t *testing.T) {
	dir := newNodeDir(t)
	ann := (&Builder{}).buildPlatformAnnotations(&build.Request{
		SourceDir: dir,
		BuildEnvs: map[string]string{
			"CNB_BUILD_SCRIPT": "build:prod",
			"BP_NODE_RUN_SCRIPTS": "should-not-override",
		},
	})
	if ann["cnb-bp-node-run-scripts"] != "build:prod" {
		t.Errorf("BP_ passthrough should not override explicit key, got %q", ann["cnb-bp-node-run-scripts"])
	}
}

// --- mock job controller ---

type mockJobCtrl struct {
	jobs       []*corev1.Pod
	getJobsErr error
	execJobErr error
	execJobFn  func(ctx context.Context, job *corev1.Pod, logger io.Writer, result *channels.RingChannel) error
	deleted    []string
}

func (m *mockJobCtrl) ExecJob(ctx context.Context, job *corev1.Pod, logger io.Writer, result *channels.RingChannel) error {
	if m.execJobFn != nil {
		return m.execJobFn(ctx, job, logger, result)
	}
	return m.execJobErr
}
func (m *mockJobCtrl) GetJob(name string) (*corev1.Pod, error)                                  { return nil, nil }
func (m *mockJobCtrl) GetServiceJobs(serviceID string) ([]*corev1.Pod, error)                   { return m.jobs, m.getJobsErr }
func (m *mockJobCtrl) DeleteJob(name string)                                                    { m.deleted = append(m.deleted, name) }
func (m *mockJobCtrl) GetLanguageBuildSetting(_ context.Context, _ code.Lang, _ string) string  { return "" }
func (m *mockJobCtrl) GetDefaultLanguageBuildSetting(_ context.Context, _ code.Lang) string     { return "" }

func newTestBuilder(ctrl *mockJobCtrl) *Builder {
	return &Builder{
		jobCtrl: ctrl,
		createAuthSecret: func(re *build.Request) (corev1.Secret, error) {
			return corev1.Secret{}, nil
		},
		deleteAuthSecret: func(re *build.Request, name string) {},
		prepareBuildKit: func(ctx context.Context, kubeClient kubernetes.Interface, namespace, cmName, imageDomain string) error {
			return nil
		},
	}
}

// --- stopPreBuildJob ---

func TestStopPreBuildJob(t *testing.T) {
	t.Run("deletes existing jobs", func(t *testing.T) {
		ctrl := &mockJobCtrl{
			jobs: []*corev1.Pod{
				{ObjectMeta: corev1.Pod{}.ObjectMeta},
			},
		}
		ctrl.jobs[0].Name = "job-1"
		b := newTestBuilder(ctrl)
		b.stopPreBuildJob(&build.Request{ServiceID: "svc1"})
		if len(ctrl.deleted) != 1 || ctrl.deleted[0] != "job-1" {
			t.Errorf("expected job-1 deleted, got %v", ctrl.deleted)
		}
	})

	t.Run("no jobs", func(t *testing.T) {
		ctrl := &mockJobCtrl{}
		b := newTestBuilder(ctrl)
		b.stopPreBuildJob(&build.Request{ServiceID: "svc1"})
		if len(ctrl.deleted) != 0 {
			t.Error("expected no deletions")
		}
	})

	t.Run("error getting jobs", func(t *testing.T) {
		ctrl := &mockJobCtrl{getJobsErr: fmt.Errorf("api error")}
		b := newTestBuilder(ctrl)
		b.stopPreBuildJob(&build.Request{ServiceID: "svc1"})
		if len(ctrl.deleted) != 0 {
			t.Error("expected no deletions on error")
		}
	})
}

// --- runCNBBuildJob ---

func TestRunCNBBuildJob(t *testing.T) {
	logger := event.GetTestLogger()

	t.Run("exec job failure", func(t *testing.T) {
		ctrl := &mockJobCtrl{execJobErr: fmt.Errorf("exec failed")}
		b := newTestBuilder(ctrl)
		dir := newNodeDir(t)
		re := &build.Request{
			ServiceID:    "svc1",
			DeployVersion: "v1",
			RbdNamespace: "ns",
			Arch:         "amd64",
			SourceDir:    dir,
			CacheDir:     "/tmp/cache",
			BuildEnvs:    map[string]string{},
			Logger:       logger,
		}
		err := b.runCNBBuildJob(re, "img:v1")
		if err == nil || !strings.Contains(err.Error(), "exec failed") {
			t.Errorf("expected exec failed error, got %v", err)
		}
	})

	t.Run("auth secret failure", func(t *testing.T) {
		ctrl := &mockJobCtrl{}
		b := &Builder{
			jobCtrl: ctrl,
			createAuthSecret: func(re *build.Request) (corev1.Secret, error) {
				return corev1.Secret{}, fmt.Errorf("auth failed")
			},
			deleteAuthSecret: func(re *build.Request, name string) {},
			prepareBuildKit: func(ctx context.Context, kubeClient kubernetes.Interface, namespace, cmName, imageDomain string) error {
				return nil
			},
		}
		dir := newNodeDir(t)
		re := &build.Request{
			ServiceID:    "svc1",
			DeployVersion: "v1",
			RbdNamespace: "ns",
			Arch:         "amd64",
			SourceDir:    dir,
			CacheDir:     "/tmp/cache",
			BuildEnvs:    map[string]string{},
			Logger:       logger,
		}
		err := b.runCNBBuildJob(re, "img:v1")
		if err == nil || !strings.Contains(err.Error(), "auth failed") {
			t.Errorf("expected auth failed error, got %v", err)
		}
	})

	t.Run("successful job with complete", func(t *testing.T) {
		ctrl := &mockJobCtrl{
			execJobFn: func(ctx context.Context, job *corev1.Pod, logger io.Writer, result *channels.RingChannel) error {
				go func() {
					result.In() <- "complete"
					result.In() <- "logcomplete"
				}()
				return nil
			},
		}
		b := newTestBuilder(ctrl)
		dir := newNodeDir(t)
		re := &build.Request{
			ServiceID:    "svc1",
			DeployVersion: "v1",
			RbdNamespace: "ns",
			Arch:         "amd64",
			SourceDir:    dir,
			CacheDir:     "/tmp/cache",
			BuildEnvs:    map[string]string{},
			Logger:       logger,
		}
		err := b.runCNBBuildJob(re, "img:v1")
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if len(ctrl.deleted) != 1 {
			t.Errorf("expected job cleanup, got %d deletions", len(ctrl.deleted))
		}
	})

	t.Run("CNB_START_SCRIPT in pod annotations and downward API", func(t *testing.T) {
		var capturedPod *corev1.Pod
		ctrl := &mockJobCtrl{
			execJobFn: func(ctx context.Context, job *corev1.Pod, logger io.Writer, result *channels.RingChannel) error {
				capturedPod = job
				go func() {
					result.In() <- "complete"
					result.In() <- "logcomplete"
				}()
				return nil
			},
		}
		b := newTestBuilder(ctrl)
		dir := newNodeDir(t)
		re := &build.Request{
			ServiceID:     "svc1",
			DeployVersion: "v1",
			RbdNamespace:  "ns",
			Arch:          "amd64",
			SourceDir:     dir,
			CacheDir:      "/tmp/cache",
			BuildEnvs:     map[string]string{"CNB_START_SCRIPT": "start:prod"},
			Logger:        logger,
		}
		err := b.runCNBBuildJob(re, "img:v1")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if capturedPod == nil {
			t.Fatal("pod was not captured")
		}

		// Verify Pod annotation
		if v, ok := capturedPod.Annotations["cnb-bp-npm-start-script"]; !ok || v != "start:prod" {
			t.Errorf("Pod annotation cnb-bp-npm-start-script = %q (exists=%v); want start:prod", v, ok)
			t.Logf("All annotations: %v", capturedPod.Annotations)
		}

		// Verify DownwardAPI volume has the env file
		foundEnvFile := false
		for _, vol := range capturedPod.Spec.Volumes {
			if vol.Name == "platform" && vol.DownwardAPI != nil {
				for _, item := range vol.DownwardAPI.Items {
					if item.Path == "env/BP_NPM_START_SCRIPT" {
						foundEnvFile = true
						if item.FieldRef.FieldPath != "metadata.annotations['cnb-bp-npm-start-script']" {
							t.Errorf("FieldPath = %q; want metadata.annotations['cnb-bp-npm-start-script']", item.FieldRef.FieldPath)
						}
					}
				}
			}
		}
		if !foundEnvFile {
			t.Error("DownwardAPI volume missing env/BP_NPM_START_SCRIPT item")
		}
	})

	t.Run("CNB_START_SCRIPT and BP_* passthrough in pod annotations and downward API", func(t *testing.T) {
		var capturedPod *corev1.Pod
		ctrl := &mockJobCtrl{
			execJobFn: func(ctx context.Context, job *corev1.Pod, logger io.Writer, result *channels.RingChannel) error {
				capturedPod = job
				go func() {
					result.In() <- "complete"
					result.In() <- "logcomplete"
				}()
				return nil
			},
		}
		b := newTestBuilder(ctrl)
		dir := newNodeDir(t)
		re := &build.Request{
			ServiceID:     "svc1",
			DeployVersion: "v1",
			RbdNamespace:  "ns",
			Arch:          "amd64",
			SourceDir:     dir,
			CacheDir:      "/tmp/cache",
			BuildEnvs: map[string]string{
				"CNB_START_SCRIPT": "start:prod",
				"BP_NODE_VERSION":  "20.10.0",
				"BP_CUSTOM_FLAG":   "enabled",
			},
			Logger: logger,
		}
		err := b.runCNBBuildJob(re, "img:v1")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if capturedPod == nil {
			t.Fatal("pod was not captured")
		}

		// Verify all annotations exist on the Pod
		wantAnnotations := map[string]string{
			"cnb-bp-npm-start-script": "start:prod",
			"cnb-bp-node-version":     "20.10.0",
			"cnb-bp-custom-flag":      "enabled",
		}
		for annKey, wantVal := range wantAnnotations {
			if got, ok := capturedPod.Annotations[annKey]; !ok {
				t.Errorf("missing annotation %q", annKey)
			} else if got != wantVal {
				t.Errorf("annotation %q = %q; want %q", annKey, got, wantVal)
			}
		}

		// Verify DownwardAPI volume has all env files
		wantEnvFiles := map[string]string{
			"env/BP_NPM_START_SCRIPT": "metadata.annotations['cnb-bp-npm-start-script']",
			"env/BP_NODE_VERSION":     "metadata.annotations['cnb-bp-node-version']",
			"env/BP_CUSTOM_FLAG":      "metadata.annotations['cnb-bp-custom-flag']",
		}
		foundFiles := make(map[string]bool)
		for _, vol := range capturedPod.Spec.Volumes {
			if vol.Name == "platform" && vol.DownwardAPI != nil {
				for _, item := range vol.DownwardAPI.Items {
					if wantField, ok := wantEnvFiles[item.Path]; ok {
						foundFiles[item.Path] = true
						if item.FieldRef.FieldPath != wantField {
							t.Errorf("DownwardAPI %s FieldPath = %q; want %q", item.Path, item.FieldRef.FieldPath, wantField)
						}
					}
				}
			}
		}
		for path := range wantEnvFiles {
			if !foundFiles[path] {
				t.Errorf("DownwardAPI volume missing %s item", path)
			}
		}
	})
}
