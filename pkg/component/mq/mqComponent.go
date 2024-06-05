// RAINBOND, Application Management Platform
// Copyright (C) 2021-2024 Goodrain Co., Ltd.

// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version. For any non-GPL usage of Rainbond,
// one or multiple Commercial Licenses authorized by Goodrain Co., Ltd.
// must be obtained first.

// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU General Public License for more details.

// You should have received a copy of the GNU General Public License
// along with this program. If not, see <http://www.gnu.org/licenses/>.

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

// New -
func New() *Component {
	defaultMqComponent = &Component{}
	return defaultMqComponent
}

// Default -
func Default() *Component {
	return defaultMqComponent
}
