package discover

import (
	"context"
	"os"
	"time"

	"github.com/goodrain/rainbond/mq/api/grpc/pb"
	"github.com/goodrain/rainbond/mq/client"
	mqcomponent "github.com/goodrain/rainbond/pkg/component/mq"
	"github.com/sirupsen/logrus"
)

const healthCheckTimeout = 2 * time.Second

var defaultHealthChecker = newHealthChecker()

type healthChecker struct {
	gate      chan struct{}
	cacheTTL  time.Duration
	checkedAt time.Time
	healthy   bool
}

func newHealthChecker() *healthChecker {
	return &healthChecker{
		gate:     make(chan struct{}, 1),
		cacheTTL: time.Second,
	}
}

// HealthCheck reports worker health for callers without a request context.
func HealthCheck() map[string]string {
	return HealthCheckContext(context.Background())
}

// HealthCheckContext checks MQ within the caller's deadline and the readiness budget.
func HealthCheckContext(ctx context.Context) map[string]string {
	component := mqcomponent.Default()
	healthy := component != nil && defaultHealthChecker.check(ctx, component.MqClient)
	if healthy {
		return map[string]string{"status": "health", "info": "worker service health"}
	}
	return map[string]string{"status": "un_health", "info": "worker service un_health"}
}

func (c *healthChecker) check(ctx context.Context, mqClient pb.TaskQueueClient) bool {
	ctx, cancel := context.WithTimeout(ctx, healthCheckTimeout)
	defer cancel()
	if mqClient == nil || ctx.Err() != nil {
		return false
	}

	// Probes and metric scrapes share a topic. Run one round trip at a time and
	// reuse its result briefly so concurrent callers cannot consume each other's messages.
	select {
	case c.gate <- struct{}{}:
		defer func() { <-c.gate }()
	case <-ctx.Done():
		return false
	}
	if ctx.Err() != nil {
		return false
	}
	if time.Since(c.checkedAt) < c.cacheTTL {
		return c.healthy
	}

	healthy := checkMQRoundTrip(ctx, mqClient)
	// A disconnected HTTP caller must not overwrite the shared health result.
	if ctx.Err() == nil {
		c.healthy = healthy
		c.checkedAt = time.Now()
	}
	return healthy
}

func checkMQRoundTrip(ctx context.Context, mqClient pb.TaskQueueClient) bool {
	_, err := mqClient.Enqueue(ctx, &pb.EnqueueRequest{
		Topic: client.WorkerHealth,
		Message: &pb.TaskMessage{
			TaskType:   "check_worker_health",
			TaskBody:   []byte(`"health check"`),
			CreateTime: time.Now().Format(time.RFC3339),
			User:       "rainbond",
			Arch:       "test",
		},
	})
	if err != nil {
		logrus.Errorf("worker check send worker topic failure: %v", err)
		return false
	}
	if ctx.Err() != nil {
		return false
	}
	hostname, _ := os.Hostname()
	message, err := mqClient.Dequeue(ctx, &pb.DequeueRequest{
		Topic:      client.WorkerHealth,
		ClientHost: hostname + "health-worker",
	})
	if err != nil {
		logrus.Errorf("failed to dequeue health check message: %v", err)
		return false
	}
	return ctx.Err() == nil && message != nil && len(message.TaskBody) != 0
}
