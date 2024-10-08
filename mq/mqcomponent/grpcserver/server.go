package grpcserver

import (
	"context"
	"fmt"
	"github.com/goodrain/rainbond/config/configs"
	"github.com/goodrain/rainbond/config/configs/rbdcomponent"
	grpcserver "github.com/goodrain/rainbond/mq/api/grpc/server"
	"github.com/goodrain/rainbond/mq/mqcomponent/mqclient"
	"github.com/goodrain/rainbond/pkg/gogo"
	"github.com/sirupsen/logrus"
	"google.golang.org/grpc"
	"net"
)

// New -
func New() *Component {
	mqConfig := configs.Default().MQConfig
	return &Component{
		mqConfig: mqConfig,
	}
}

// Component -
type Component struct {
	server   *grpc.Server
	lis      net.Listener
	mqConfig *rbdcomponent.MQConfig
}

// StartCancel -
func (c *Component) StartCancel(ctx context.Context, cancel context.CancelFunc) error {
	s := grpc.NewServer()
	c.server = s
	grpcserver.RegisterServer(s, mqclient.Default().ActionMQ())
	return gogo.Go(func(ctx context.Context) (err error) {
		defer cancel()
		c.lis, err = net.Listen("tcp", fmt.Sprintf(":%d", c.mqConfig.APIPort))
		logrus.Infof("grpc server listen on %d", c.mqConfig.APIPort)
		if err := s.Serve(c.lis); err != nil {
			logrus.Error("mq api grpc listen error.", err.Error())
			return err
		}
		return err
	})
}

// Start -
func (c *Component) Start(ctx context.Context) (err error) {
	panic("implement me")
}

// CloseHandle -
func (c *Component) CloseHandle() {
	err := c.lis.Close()
	if err != nil {
		logrus.Errorf("failed to close listener: %v", err)
	}
}
