package clean

import "testing"

// TestAutoClean 执行此方法你应该通过第三方组件将rbd-hub暴露出来，并且通过kubectl命令查找仓库账号密码
func TestAutoClean(t *testing.T) {
	t.Skip("integration test requires live registry access")
}
