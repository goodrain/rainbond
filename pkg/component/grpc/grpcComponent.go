package grpc

import (
	"context"
	"github.com/goodrain/rainbond/config/configs"
	"github.com/goodrain/rainbond/worker/client"
)

var defaultGrpcComponent *Component

type Component struct {
	StatusClient *client.AppRuntimeSyncClient
}

func (c Component) Start(ctx context.Context, cfg *configs.Config) (err error) {
	c.StatusClient, err = client.NewClient(ctx, cfg.APIConfig.RbdWorker)
	return err
}

func (c Component) CloseHandle() {
}

func Grpc() *Component {
	defaultGrpcComponent = &Component{}
	return &Component{}
}

func Default() *Component {
	return defaultGrpcComponent
}
