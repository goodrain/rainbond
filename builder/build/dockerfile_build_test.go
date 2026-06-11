package build

import (
	"strings"
	"testing"
)

// TestLocalBuildKitCacheArgsContainServiceID 验证生成的本地 layer cache 参数
// 同时包含 --cache-from 与 --cache-to，且缓存路径包含 serviceID。
func TestLocalBuildKitCacheArgsContainServiceID(t *testing.T) {
	const serviceID = "svc-abc123"

	args := localBuildKitCacheArgs(serviceID)

	from := flagValue(t, args, "--cache-from")
	to := flagValue(t, args, "--cache-to")

	wantDir := "/cache/buildkit/" + serviceID
	if !strings.Contains(from, wantDir) {
		t.Errorf("--cache-from %q does not contain cache dir %q", from, wantDir)
	}
	if !strings.Contains(to, wantDir) {
		t.Errorf("--cache-to %q does not contain cache dir %q", to, wantDir)
	}
	if !strings.Contains(from, "type=local") || !strings.Contains(from, "src=") {
		t.Errorf("--cache-from %q is not a local source spec", from)
	}
	if !strings.Contains(to, "type=local") || !strings.Contains(to, "dest=") || !strings.Contains(to, "mode=max") {
		t.Errorf("--cache-to %q is not a local mode=max dest spec", to)
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
