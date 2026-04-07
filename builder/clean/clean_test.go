package clean

import (
	"context"
	"testing"
	"time"

	"github.com/goodrain/rainbond/builder/sources"
	"k8s.io/client-go/kubernetes/fake"
)

// capability_id: rainbond.image-clean.registry-gc-noop
func TestPodExecCmdNoMatchingPod(t *testing.T) {
	manager := &Manager{}
	clientset := fake.NewSimpleClientset()

	stdout, stderr, err := manager.PodExecCmd(nil, clientset, "rbd-hub", []string{"registry", "garbage-collect"})
	if err != nil {
		t.Fatal(err)
	}
	if stdout.Len() != 0 || stderr.Len() != 0 {
		t.Fatalf("expected empty output, got stdout=%q stderr=%q", stdout.String(), stderr.String())
	}
}

// capability_id: rainbond.image-clean.stop-loop
func TestManagerStopCancelsContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	manager := &Manager{
		ctx:         ctx,
		cancel:      cancel,
		imageClient: sources.ImageClient(nil),
	}

	if err := manager.Stop(); err != nil {
		t.Fatal(err)
	}

	select {
	case <-ctx.Done():
	case <-time.After(time.Second):
		t.Fatal("expected context cancellation")
	}

}

// TestAutoClean 执行此方法你应该通过第三方组件将rbd-hub暴露出来，并且通过kubectl命令查找仓库账号密码
func TestAutoClean(t *testing.T) {
	t.Skip("integration test requires live registry access")
}