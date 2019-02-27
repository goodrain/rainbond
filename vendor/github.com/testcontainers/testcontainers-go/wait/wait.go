package wait

import (
	"context"
	"time"

	"github.com/docker/go-connections/nat"
)

type Strategy interface {
	WaitUntilReady(context.Context, StrategyTarget) error
}

type StrategyTarget interface {
	Host(context.Context) (string, error)
	MappedPort(context.Context, nat.Port) (nat.Port, error)
}

func defaultStartupTimeout() time.Duration {
	return 60 * time.Second
}
