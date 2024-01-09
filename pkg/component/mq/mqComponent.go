package mq

import (
	"context"
	"github.com/goodrain/rainbond/config/configs"
	"github.com/goodrain/rainbond/mq/client"
)

var defaultMqComponent *Component

type Component struct {
	MqClient client.MQClient
}

func (c *Component) Start(ctx context.Context, cfg *configs.Config) error {
	mqClient, err := client.NewMqClient(cfg.APIConfig.MQAPI)
	c.MqClient = mqClient
	return err
}

func (c *Component) CloseHandle() {
}

func MQ() *Component {
	defaultMqComponent = &Component{}
	return defaultMqComponent
}

func Default() *Component {
	return defaultMqComponent
}
