package mqclient

import (
	"context"
	"github.com/goodrain/rainbond/config/configs"
	"github.com/goodrain/rainbond/mq/api/mq"
)

var defaultMQClientComponent *Component

// Component -
type Component struct {
	actionMQ mq.ActionMQ
}

// Default -
func Default() *Component {
	return defaultMQClientComponent
}

// ActionMQ -
func (m *Component) ActionMQ() mq.ActionMQ {
	return m.actionMQ
}

// Start -
func (m *Component) Start(ctx context.Context, cfg *configs.Config) error {
	m.actionMQ = mq.NewActionMQ(ctx, cfg.MQConfig)
	return m.actionMQ.Start()
}

// CloseHandle -
func (m *Component) CloseHandle() {
}

// MQClient -
func MQClient() *Component {
	defaultMQClientComponent = &Component{}
	return defaultMQClientComponent
}
