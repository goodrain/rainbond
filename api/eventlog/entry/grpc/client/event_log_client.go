// Copyright (C) 2014-2018 Goodrain Co., Ltd.
// RAINBOND, Application Management Platform

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

package client

import (
	"context"
	"time"

	"github.com/goodrain/rainbond/api/eventlog/entry/grpc/pb"
	grpc1 "google.golang.org/grpc"
	"google.golang.org/grpc/backoff"
	"google.golang.org/grpc/keepalive"
)

// NewEventClient new a event client
func NewEventClient(ctx context.Context, server string) (pb.EventLogClient, error) {
	// 配置 keepalive 参数
	kaParams := keepalive.ClientParameters{
		Time:                10 * time.Second, // 每 10 秒发送心跳
		Timeout:             3 * time.Second,  // 心跳超时时间
		PermitWithoutStream: true,             // 无活动流时也发送心跳
	}

	// 配置 backoff 参数，加快服务重启后的重连速度
	backoffConfig := backoff.Config{
		BaseDelay:  1 * time.Second,  // 初始重试延迟
		Multiplier: 1.5,              // 指数退避倍数
		Jitter:     0.2,              // 20% 随机抖动
		MaxDelay:   10 * time.Second, // 最大重试延迟
	}

	conn, err := grpc1.DialContext(ctx, server,
		grpc1.WithInsecure(),
		grpc1.WithKeepaliveParams(kaParams),
		grpc1.WithConnectParams(grpc1.ConnectParams{
			Backoff:           backoffConfig,
			MinConnectTimeout: 5 * time.Second,
		}),
	)
	if err != nil {
		return nil, err
	}
	return pb.NewEventLogClient(conn), nil
}
