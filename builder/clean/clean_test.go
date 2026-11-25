package clean

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/goodrain/rainbond/builder/sources"
	"github.com/goodrain/rainbond/config/configs"
	guard "github.com/goodrain/rainbond/pkg/cleanup"
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

func cleanupTestConfiguration(t *testing.T, keep int, clock, zone string) {
	t.Helper()
	config := configs.Default().ChaosConfig
	saved := *config
	t.Cleanup(func() { *config = saved })
	config.KeepCount = keep
	config.CleanTime = clock
	config.CleanTimeZone = zone
}

// capability_id: rainbond.image-clean.coordinated-manager-required
func TestManagerRequiresCoordinatedRunner(t *testing.T) {
	cleanupTestConfiguration(t, 5, "03:00", "Asia/Shanghai")
	manager, err := CreateCleanManager(nil)
	if err != nil {
		t.Fatal(err)
	}
	defer manager.Stop()
	if err := manager.Start(make(chan error, 1)); err == nil {
		t.Fatal("manager started without a coordinated runner")
	}
}

// capability_id: rainbond.image-clean.coordinated-manager-config
func TestManagerRejectsInvalidDailyConfiguration(t *testing.T) {
	tests := []struct {
		name        string
		keep        int
		clock, zone string
	}{
		{"zero keep", 0, "03:00", "Asia/Shanghai"},
		{"negative keep", -1, "03:00", "Asia/Shanghai"},
		{"invalid clock", 5, "3:00", "Asia/Shanghai"},
		{"invalid zone", 5, "03:00", "Invalid/Zone"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cleanupTestConfiguration(t, tt.keep, tt.clock, tt.zone)
			if manager, err := CreateCleanManager(nil, func(context.Context, string, uint) error { return nil }); err == nil {
				manager.Stop()
				t.Fatal("invalid cleanup configuration accepted")
			}
		})
	}
}

// capability_id: rainbond.image-clean.coordinated-manager-triggers
func TestManagerRunsCoordinatedStartupAndDailyWithoutFatalErrors(t *testing.T) {
	cleanupTestConfiguration(t, 7, "03:00", "Asia/Shanghai")
	type call struct {
		trigger string
		keep    uint
	}
	calls := make(chan call, 3)
	manager, err := CreateCleanManager(nil, func(ctx context.Context, trigger string, keep uint) error {
		calls <- call{trigger, keep}
		return errors.New("cleanup prerequisites unavailable")
	})
	if err != nil {
		t.Fatal(err)
	}
	defer manager.Stop()
	location, err := time.LoadLocation("Asia/Shanghai")
	if err != nil {
		t.Fatal(err)
	}
	current := time.Date(2026, 11, 25, 2, 0, 0, 0, location)
	manager.now = func() time.Time { return current }
	waiting := make(chan struct{})
	stopped := make(chan struct{})
	waits := 0
	manager.wait = func(ctx context.Context, duration time.Duration) error {
		waits++
		if waits == 1 {
			current = current.Add(duration)
			return nil
		}
		close(waiting)
		<-ctx.Done()
		close(stopped)
		return ctx.Err()
	}
	errorsToChaos := make(chan error, 1)
	if err := manager.Start(errorsToChaos); err != nil {
		t.Fatal(err)
	}
	if err := manager.Start(errorsToChaos); err == nil {
		t.Fatal("duplicate Start created another startup job")
	}
	for _, want := range []string{"startup", "daily:2026-11-25T03:00:00+08:00"} {
		select {
		case got := <-calls:
			if got.trigger != want || got.keep != 7 {
				t.Fatalf("runner call=%+v; want %s keep=7", got, want)
			}
		case <-time.After(5 * time.Second):
			t.Fatal("coordinated runner did not execute")
		}
	}
	select {
	case <-waiting:
	case <-time.After(5 * time.Second):
		t.Fatal("daily loop did not wait")
	}
	if err := manager.Stop(); err != nil {
		t.Fatal(err)
	}
	select {
	case <-stopped:
	case <-time.After(5 * time.Second):
		t.Fatal("stop did not cancel scheduled wait")
	}
	select {
	case err := <-errorsToChaos:
		t.Fatalf("controlled runner failure escaped to Chaos: %v", err)
	default:
	}
	select {
	case extra := <-calls:
		t.Fatalf("cleanup executed after stop: %+v", extra)
	default:
	}
}

// capability_id: rainbond.image-clean.coordinated-manager-cancelled-start
func TestManagerStoppedBeforeStartDoesNotExecute(t *testing.T) {
	cleanupTestConfiguration(t, 5, "03:00", "Asia/Shanghai")
	called := false
	manager, err := CreateCleanManager(nil, func(context.Context, string, uint) error { called = true; return nil })
	if err != nil {
		t.Fatal(err)
	}
	manager.Stop()
	if err := manager.Start(make(chan error, 1)); !errors.Is(err, context.Canceled) {
		t.Fatalf("start after stop error=%v", err)
	}
	if called {
		t.Fatal("cleanup executed after prior cancellation")
	}
}

// capability_id: rainbond.image-clean.scheduler-wait-cancellation
func TestCleanupWaitHonorsCancellationAndDeadline(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := waitForCleanupTime(ctx, time.Hour); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancelled wait error=%v", err)
	}
	if err := waitForCleanupTime(context.Background(), 0); err != nil {
		t.Fatalf("elapsed wait error=%v", err)
	}
}

// capability_id: rainbond.image-clean.coordinated-manager-config
func TestManagerRejectsMultipleCoordinatedRunners(t *testing.T) {
	cleanupTestConfiguration(t, 5, "03:00", "Asia/Shanghai")
	runner := func(context.Context, string, uint) error { return nil }
	if manager, err := CreateCleanManager(nil, runner, runner); err == nil {
		manager.Stop()
		t.Fatal("ambiguous cleanup runners accepted")
	}
}

// capability_id: rainbond.image-clean.coordinated-manager-busy-retry
func TestManagerRetriesBusyWithSameTrigger(t *testing.T) {
	cleanupTestConfiguration(t, 5, "03:00", "Asia/Shanghai")
	trigger := "daily:2026-11-25T03:00:00+08:00"
	var triggers []string
	var waits []time.Duration
	manager, err := CreateCleanManager(nil, func(ctx context.Context, id string, keep uint) error {
		triggers = append(triggers, id)
		if keep != 5 {
			t.Fatalf("keepCount=%d; want 5", keep)
		}
		if len(triggers) < 3 {
			return fmt.Errorf("active writer: %w", guard.ErrCoordinationBusy)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	defer manager.Stop()
	manager.wait = func(ctx context.Context, d time.Duration) error { waits = append(waits, d); return nil }
	manager.runTrigger(manager.ctx, trigger)
	if len(triggers) != 3 || len(waits) != 2 {
		t.Fatalf("runs/waits=%d/%d; want 3/2", len(triggers), len(waits))
	}
	for _, got := range triggers {
		if got != trigger {
			t.Fatalf("busy retry changed trigger: %q", got)
		}
	}
	for _, got := range waits {
		if got != time.Minute {
			t.Fatalf("busy retry delay=%s; want one minute", got)
		}
	}
}

// capability_id: rainbond.image-clean.coordinated-manager-busy-cancel
func TestManagerCancelsBusyBackoff(t *testing.T) {
	for _, waitReturnsError := range []bool{false, true} {
		name := "timer races cancellation"
		if waitReturnsError {
			name = "wait reports cancellation"
		}
		t.Run(name, func(t *testing.T) {
			cleanupTestConfiguration(t, 5, "03:00", "Asia/Shanghai")
			calls, waits := 0, 0
			manager, err := CreateCleanManager(nil, func(context.Context, string, uint) error { calls++; return guard.ErrCoordinationBusy })
			if err != nil {
				t.Fatal(err)
			}
			manager.wait = func(ctx context.Context, d time.Duration) error {
				waits++
				if d != time.Minute {
					t.Fatalf("busy wait=%s", d)
				}
				manager.Stop()
				if waitReturnsError {
					return ctx.Err()
				}
				return nil // An expiring timer may race with cancellation.
			}
			manager.runTrigger(manager.ctx, "startup")
			if calls != 1 || waits != 1 {
				t.Fatalf("cleanup continued after cancellation: calls=%d waits=%d", calls, waits)
			}
		})
	}
}

// capability_id: rainbond.image-clean.coordinated-manager-unavailable-no-retry
func TestManagerDoesNotRetryUnavailable(t *testing.T) {
	cleanupTestConfiguration(t, 5, "03:00", "Asia/Shanghai")
	calls, waits := 0, 0
	manager, err := CreateCleanManager(nil, func(context.Context, string, uint) error { calls++; return guard.ErrCoordinationUnavailable })
	if err != nil {
		t.Fatal(err)
	}
	defer manager.Stop()
	manager.wait = func(context.Context, time.Duration) error { waits++; return nil }
	manager.runTrigger(manager.ctx, "startup")
	if calls != 1 || waits != 0 {
		t.Fatalf("uncertified cleanup retried: calls=%d waits=%d", calls, waits)
	}
}
