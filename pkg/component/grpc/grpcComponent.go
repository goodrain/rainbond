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

package grpc

import (
	"context"
	"github.com/goodrain/rainbond/config/configs"
	"github.com/goodrain/rainbond/worker/client"
)

var defaultGrpcComponent *Component

// Component -
type Component struct {
	StatusClient *client.AppRuntimeSyncClient
}

// Start -
func (c *Component) Start(ctx context.Context, cfg *configs.Config) (err error) {
	c.StatusClient, err = client.NewClient(ctx, cfg.APIConfig.RbdWorker)
	return err
}

// CloseHandle -
func (c *Component) CloseHandle() {
}

// Grpc -
func Grpc() *Component {
	defaultGrpcComponent = &Component{}
	return defaultGrpcComponent
}

// Default -
func Default() *Component {
	return defaultGrpcComponent
}
