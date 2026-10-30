package discover

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/goodrain/rainbond/mq/api/grpc/pb"
	"github.com/goodrain/rainbond/mq/client"
	mqcomponent "github.com/goodrain/rainbond/pkg/component/mq"
	"google.golang.org/grpc"
)

type healthTestMQ struct {
	pb.TaskQueueClient
	enqueue func(context.Context, *pb.EnqueueRequest) (*pb.TaskReply, error)
	dequeue func(context.Context, *pb.DequeueRequest) (*pb.TaskMessage, error)
}

func (m *healthTestMQ) Enqueue(ctx context.Context, request *pb.EnqueueRequest, _ ...grpc.CallOption) (*pb.TaskReply, error) {
	return m.enqueue(ctx, request)
}

func (m *healthTestMQ) Dequeue(ctx context.Context, request *pb.DequeueRequest, _ ...grpc.CallOption) (*pb.TaskMessage, error) {
	return m.dequeue(ctx, request)
}

func (m *healthTestMQ) SendBuilderTopic(task client.TaskStruct) error {
	_, err := m.Enqueue(context.Background(), &pb.EnqueueRequest{Topic: task.Topic})
	return err
}

func (m *healthTestMQ) Close() {}

// capability_id: rainbond.worker.bounded-mq-health-check
func TestHealthCheckReturnsPromptlyWhenMQIsHealthy(t *testing.T) {
	component := mqcomponent.Default()
	if component == nil {
		component = mqcomponent.New()
	}
	previous := component.MqClient
	t.Cleanup(func() { component.MqClient = previous })
	component.MqClient = &healthTestMQ{
		enqueue: func(context.Context, *pb.EnqueueRequest) (*pb.TaskReply, error) {
			return &pb.TaskReply{Status: "success"}, nil
		},
		dequeue: func(context.Context, *pb.DequeueRequest) (*pb.TaskMessage, error) {
			return &pb.TaskMessage{TaskBody: []byte("health check")}, nil
		},
	}
	started := time.Now()
	result := HealthCheck()
	if result["status"] != "health" {
		t.Fatalf("healthy MQ reported unhealthy: %v", result)
	}
	if elapsed := time.Since(started); elapsed > 500*time.Millisecond {
		t.Fatalf("healthy MQ check consumed the readiness budget: %v", elapsed)
	}
}

// capability_id: rainbond.worker.bounded-mq-health-check
func TestHealthCheckUsesOneDeadlineForEnqueueAndDequeue(t *testing.T) {
	checker := newHealthChecker()
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	deadline, _ := ctx.Deadline()
	mq := &healthTestMQ{
		enqueue: func(ctx context.Context, request *pb.EnqueueRequest) (*pb.TaskReply, error) {
			actual, ok := ctx.Deadline()
			if !ok || !actual.Equal(deadline) {
				t.Fatalf("enqueue did not inherit caller deadline: %v", actual)
			}
			if request.Topic != client.WorkerHealth || request.Message == nil || len(request.Message.TaskBody) == 0 {
				t.Fatalf("invalid health message: %v", request)
			}
			return &pb.TaskReply{}, nil
		},
		dequeue: func(ctx context.Context, request *pb.DequeueRequest) (*pb.TaskMessage, error) {
			actual, ok := ctx.Deadline()
			if !ok || !actual.Equal(deadline) || request.Topic != client.WorkerHealth {
				t.Fatalf("dequeue did not share enqueue budget: %v", actual)
			}
			return &pb.TaskMessage{TaskBody: []byte("health check")}, nil
		},
	}
	if !checker.check(ctx, mq) {
		t.Fatal("healthy round trip failed")
	}
}

// capability_id: rainbond.worker.bounded-mq-health-check
func TestHealthCheckBoundsBackgroundCaller(t *testing.T) {
	checker := newHealthChecker()
	started := time.Now()
	mq := &healthTestMQ{
		enqueue: func(ctx context.Context, _ *pb.EnqueueRequest) (*pb.TaskReply, error) {
			deadline, ok := ctx.Deadline()
			if !ok || deadline.After(started.Add(2100*time.Millisecond)) {
				t.Fatalf("health check exceeds the legacy 3s readiness budget: %v", deadline)
			}
			return &pb.TaskReply{}, nil
		},
		dequeue: func(context.Context, *pb.DequeueRequest) (*pb.TaskMessage, error) {
			return &pb.TaskMessage{TaskBody: []byte("health check")}, nil
		},
	}
	if !checker.check(context.Background(), mq) {
		t.Fatal("healthy round trip failed")
	}
}

// capability_id: rainbond.worker.bounded-mq-health-check
func TestHealthCheckStopsOnCallerCancellation(t *testing.T) {
	for _, phase := range []string{"enqueue", "dequeue"} {
		t.Run(phase, func(t *testing.T) {
			checker := newHealthChecker()
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			started := make(chan struct{})
			mq := &healthTestMQ{
				enqueue: func(ctx context.Context, _ *pb.EnqueueRequest) (*pb.TaskReply, error) {
					if phase == "enqueue" {
						close(started)
						<-ctx.Done()
						return nil, ctx.Err()
					}
					return &pb.TaskReply{}, nil
				},
				dequeue: func(ctx context.Context, _ *pb.DequeueRequest) (*pb.TaskMessage, error) {
					close(started)
					<-ctx.Done()
					return nil, ctx.Err()
				},
			}
			done := make(chan bool, 1)
			go func() { done <- checker.check(ctx, mq) }()
			<-started
			cancel()
			select {
			case healthy := <-done:
				if healthy {
					t.Fatal("canceled check reported healthy")
				}
			case <-time.After(500 * time.Millisecond):
				t.Fatal("health check continued after caller cancellation")
			}
		})
	}
}

// capability_id: rainbond.worker.bounded-mq-health-check
func TestHealthCheckCoalescesConcurrentProbes(t *testing.T) {
	checker := newHealthChecker()
	var sends atomic.Int32
	var receives atomic.Int32
	started := make(chan struct{})
	release := make(chan struct{})
	mq := &healthTestMQ{
		enqueue: func(context.Context, *pb.EnqueueRequest) (*pb.TaskReply, error) {
			if sends.Add(1) == 1 {
				close(started)
			}
			<-release
			return &pb.TaskReply{}, nil
		},
		dequeue: func(context.Context, *pb.DequeueRequest) (*pb.TaskMessage, error) {
			receives.Add(1)
			return &pb.TaskMessage{TaskBody: []byte("health check")}, nil
		},
	}
	done := make(chan bool, 16)
	for i := 0; i < cap(done); i++ {
		go func() { done <- checker.check(context.Background(), mq) }()
	}
	<-started
	close(release)
	for i := 0; i < cap(done); i++ {
		if !<-done {
			t.Fatal("concurrent health check failed")
		}
	}
	if sends.Load() != 1 || receives.Load() != 1 {
		t.Fatalf("probes competed for health messages: sends=%d receives=%d", sends.Load(), receives.Load())
	}
}

// capability_id: rainbond.worker.bounded-mq-health-check
func TestHealthCheckCanceledWaiterDoesNotStartAnotherRoundTrip(t *testing.T) {
	checker := newHealthChecker()
	started := make(chan struct{})
	release := make(chan struct{})
	var sends atomic.Int32
	mq := &healthTestMQ{
		enqueue: func(context.Context, *pb.EnqueueRequest) (*pb.TaskReply, error) {
			sends.Add(1)
			close(started)
			<-release
			return &pb.TaskReply{}, nil
		},
		dequeue: func(context.Context, *pb.DequeueRequest) (*pb.TaskMessage, error) {
			return &pb.TaskMessage{TaskBody: []byte("health check")}, nil
		},
	}
	done := make(chan bool, 1)
	go func() { done <- checker.check(context.Background(), mq) }()
	<-started
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	if checker.check(ctx, mq) {
		t.Error("expired waiter reported healthy")
	}
	close(release)
	if !<-done || sends.Load() != 1 {
		t.Fatal("expired waiter affected the active health check")
	}
	if checker.check(ctx, mq) {
		t.Fatal("canceled caller received cached success")
	}
}

// capability_id: rainbond.worker.bounded-mq-health-check
func TestHealthCheckRejectsFailedOrEmptyRoundTrips(t *testing.T) {
	for _, phase := range []string{"missing-client", "enqueue", "dequeue", "nil-message", "empty-message"} {
		t.Run(phase, func(t *testing.T) {
			checker := newHealthChecker()
			var sends, receives int
			mq := &healthTestMQ{
				enqueue: func(context.Context, *pb.EnqueueRequest) (*pb.TaskReply, error) {
					sends++
					if phase == "enqueue" {
						return nil, errors.New("enqueue unavailable")
					}
					return &pb.TaskReply{}, nil
				},
				dequeue: func(context.Context, *pb.DequeueRequest) (*pb.TaskMessage, error) {
					receives++
					if phase == "dequeue" {
						return nil, errors.New("EOF")
					}
					if phase == "nil-message" {
						return nil, nil
					}
					return &pb.TaskMessage{}, nil
				},
			}
			var healthy bool
			if phase == "missing-client" {
				healthy = checker.check(context.Background(), nil)
			} else {
				healthy = checker.check(context.Background(), mq)
			}
			if healthy || sends > 1 || receives > 1 {
				t.Fatalf("invalid health result or repeated dequeue: healthy=%v sends=%d receives=%d", healthy, sends, receives)
			}
		})
	}
}

// capability_id: rainbond.worker.bounded-mq-health-check
func TestHealthCheckRecoversAfterTransientMQFailure(t *testing.T) {
	checker := newHealthChecker()
	checker.cacheTTL = 10 * time.Millisecond
	calls := 0
	mq := &healthTestMQ{
		enqueue: func(context.Context, *pb.EnqueueRequest) (*pb.TaskReply, error) {
			calls++
			if calls == 1 {
				return nil, errors.New("transient EOF")
			}
			return &pb.TaskReply{}, nil
		},
		dequeue: func(context.Context, *pb.DequeueRequest) (*pb.TaskMessage, error) {
			return &pb.TaskMessage{TaskBody: []byte("health check")}, nil
		},
	}
	if checker.check(context.Background(), mq) || checker.check(context.Background(), mq) || calls != 1 {
		t.Fatal("concurrent callers did not reuse the failed check")
	}
	time.Sleep(20 * time.Millisecond)
	if !checker.check(context.Background(), mq) || calls != 2 {
		t.Fatal("health check did not recover after MQ recovered")
	}
}

// capability_id: rainbond.worker.bounded-mq-health-check
func TestHealthCheckCanceledRoundTripDoesNotCacheFailure(t *testing.T) {
	checker := newHealthChecker()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	sends, receives := 0, 0
	mq := &healthTestMQ{
		enqueue: func(context.Context, *pb.EnqueueRequest) (*pb.TaskReply, error) {
			sends++
			if sends == 1 {
				cancel()
			}
			return &pb.TaskReply{}, nil
		},
		dequeue: func(context.Context, *pb.DequeueRequest) (*pb.TaskMessage, error) {
			receives++
			return &pb.TaskMessage{TaskBody: []byte("health check")}, nil
		},
	}
	if checker.check(ctx, mq) || receives != 0 {
		t.Fatal("canceled enqueue proceeded to consume another health message")
	}
	if !checker.check(context.Background(), mq) || sends != 2 || receives != 1 {
		t.Fatal("canceled caller cached an unhealthy result for subsequent probes")
	}
}

// capability_id: rainbond.worker.bounded-mq-health-check
func TestHealthCheckContextReportsMissingMQ(t *testing.T) {
	component := mqcomponent.Default()
	if component == nil {
		component = mqcomponent.New()
	}
	previous := component.MqClient
	t.Cleanup(func() { component.MqClient = previous })
	component.MqClient = nil
	result := HealthCheckContext(context.Background())
	if result["status"] != "un_health" || result["info"] != "worker service un_health" {
		t.Fatalf("missing MQ client did not report unhealthy: %v", result)
	}
}
