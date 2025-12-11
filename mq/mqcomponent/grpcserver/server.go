package grpcserver

import (
	"context"
	"fmt"
	"net"
	"time"

	"github.com/goodrain/rainbond/config/configs"
	"github.com/goodrain/rainbond/config/configs/rbdcomponent"
	grpcserver "github.com/goodrain/rainbond/mq/api/grpc/server"
	"github.com/goodrain/rainbond/mq/mqcomponent/mqclient"
	"github.com/goodrain/rainbond/pkg/gogo"
	"github.com/sirupsen/logrus"
	"google.golang.org/grpc"
	"google.golang.org/grpc/keepalive"
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
	// 配置 keepalive 策略，允许客户端发送 keepalive ping
	kaEnforcementPolicy := keepalive.EnforcementPolicy{
		MinTime:             5 * time.Second, // 客户端发送 keepalive ping 的最小间隔
		PermitWithoutStream: true,            // 允许没有活跃 stream 时发送 keepalive
	}

	// 配置服务端 keepalive 参数
	kaServerParams := keepalive.ServerParameters{
		MaxConnectionIdle:     30 * time.Second,  // 空闲连接最大存活时间
		MaxConnectionAge:      60 * time.Minute,  // 连接最大存活时间
		MaxConnectionAgeGrace: 5 * time.Second,   // 关闭连接前的宽限期
		Time:                  10 * time.Second,  // 服务端发送 keepalive ping 的间隔
		Timeout:               3 * time.Second,   // 等待 keepalive ping 响应的超时时间
	}

	s := grpc.NewServer(
		grpc.KeepaliveEnforcementPolicy(kaEnforcementPolicy),
		grpc.KeepaliveParams(kaServerParams),
	)
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
