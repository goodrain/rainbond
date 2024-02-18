package grpcserver

import (
	"context"
	"fmt"
	"github.com/goodrain/rainbond/config/configs"
	grpcserver "github.com/goodrain/rainbond/mq/api/grpc/server"
	"github.com/goodrain/rainbond/mq/mqcomponent/mqclient"
	"github.com/goodrain/rainbond/pkg/gogo"
	"github.com/sirupsen/logrus"
	"google.golang.org/grpc"
	"net"
)

// NewGrpcServer -
func NewGrpcServer() *Component {
	return &Component{}
}

// Component -
type Component struct {
	server *grpc.Server
	lis    net.Listener
}

// StartCancel -
func (c *Component) StartCancel(ctx context.Context, cancel context.CancelFunc, cfg *configs.Config) error {
	s := grpc.NewServer()
	c.server = s
	grpcserver.RegisterServer(s, mqclient.Default().ActionMQ())
	return gogo.Go(func(ctx context.Context) (err error) {
		defer cancel()
		c.lis, err = net.Listen("tcp", fmt.Sprintf(":%d", cfg.MQConfig.APIPort))
		logrus.Infof("grpc server listen on %d", cfg.MQConfig.APIPort)
		if err := s.Serve(c.lis); err != nil {
			logrus.Error("mq api grpc listen error.", err.Error())
			return err
		}
		return err
	})
}

// Start -
func (c *Component) Start(ctx context.Context, cfg *configs.Config) (err error) {
	panic("implement me")
}

// CloseHandle -
func (c *Component) CloseHandle() {
	err := c.lis.Close()
	if err != nil {
		logrus.Errorf("failed to close listener: %v", err)
	}
}
