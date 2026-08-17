package build

import (
	"reflect"
	"strings"
	"testing"
)

// capability_id: rainbond.dockerfile-build.proxy-env-inheritance
func TestBuildKitProxyEnvVars(t *testing.T) {
	tests := []struct {
		name string
		env  map[string]string
		want map[string]string
	}{
		{
			name: "inherits configured proxy variables only",
			env: map[string]string{
				"HTTP_PROXY":    "http://proxy.example:8080",
				"HTTPS_PROXY":   "http://proxy.example:8443",
				"NO_PROXY":      "localhost,127.0.0.1",
				"UNRELATED_ENV": "must-not-be-propagated",
			},
			want: map[string]string{
				"HTTP_PROXY":  "http://proxy.example:8080",
				"HTTPS_PROXY": "http://proxy.example:8443",
				"NO_PROXY":    "localhost,127.0.0.1",
			},
		},
		{
			name: "leaves buildkit environment unchanged without proxy configuration",
			env: map[string]string{
				"UNRELATED_ENV": "must-not-be-propagated",
			},
			want: map[string]string{},
		},
		{
			name: "skips empty proxy variables",
			env: map[string]string{
				"HTTP_PROXY":  "http://proxy.example:8080",
				"HTTPS_PROXY": "",
				"NO_PROXY":    "",
			},
			want: map[string]string{
				"HTTP_PROXY": "http://proxy.example:8080",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			for _, name := range []string{"HTTP_PROXY", "HTTPS_PROXY", "NO_PROXY"} {
				t.Setenv(name, "")
			}
			for name, value := range tt.env {
				t.Setenv(name, value)
			}

			got := make(map[string]string)
			for _, env := range buildKitProxyEnvVars() {
				got[env.Name] = env.Value
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("buildKitProxyEnvVars() = %#v, want %#v", got, tt.want)
			}
		})
	}
}

// TestLocalBuildKitCacheArgsContainServiceID 验证生成的本地 layer cache 参数
// 使用 buildctl 的 --import-cache / --export-cache（而非 buildx 的
// --cache-from / --cache-to），且缓存路径包含 serviceID。
func TestLocalBuildKitCacheArgsContainServiceID(t *testing.T) {
	const serviceID = "svc-abc123"

	args := localBuildKitCacheArgs(serviceID)

	for _, invalid := range []string{"--cache-from", "--cache-to"} {
		for _, a := range args {
			if a == invalid {
				t.Errorf("args contain %q, which buildctl does not support", invalid)
			}
		}
	}

	from := flagValue(t, args, "--import-cache")
	to := flagValue(t, args, "--export-cache")

	wantDir := "/cache/buildkit/" + serviceID
	if !strings.Contains(from, wantDir) {
		t.Errorf("--import-cache %q does not contain cache dir %q", from, wantDir)
	}
	if !strings.Contains(to, wantDir) {
		t.Errorf("--export-cache %q does not contain cache dir %q", to, wantDir)
	}
	if !strings.Contains(from, "type=local") || !strings.Contains(from, "src=") {
		t.Errorf("--import-cache %q is not a local source spec", from)
	}
	if !strings.Contains(to, "type=local") || !strings.Contains(to, "dest=") || !strings.Contains(to, "mode=max") {
		t.Errorf("--export-cache %q is not a local mode=max dest spec", to)
	}
}

// flagValue 返回 args 中紧跟在 flag 之后的值，找不到则 fail。
func flagValue(t *testing.T, args []string, flag string) string {
	t.Helper()
	for i, a := range args {
		if a == flag {
			if i+1 >= len(args) {
				t.Fatalf("flag %q has no following value in args %#v", flag, args)
			}
			return args[i+1]
		}
	}
	t.Fatalf("flag %q not found in args %#v", flag, args)
	return ""
}
