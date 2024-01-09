package mq

import (
	"context"
	"github.com/goodrain/rainbond/config/configs"
	"github.com/goodrain/rainbond/mq/client"
)

var defaultMqComponent *Component

// Component -
type Component struct {
	MqClient client.MQClient
}

// Start -
func (c *Component) Start(ctx context.Context, cfg *configs.Config) error {
	mqClient, err := client.NewMqClient(cfg.APIConfig.MQAPI)
	c.MqClient = mqClient
	return err
}

// CloseHandle -
func (c *Component) CloseHandle() {
}

// MQ -
func MQ() *Component {
	defaultMqComponent = &Component{}
	return defaultMqComponent
}

// Default -
func Default() *Component {
	return defaultMqComponent
}
