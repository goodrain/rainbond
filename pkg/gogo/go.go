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

package gogo

import (
	"context"
	"github.com/sirupsen/logrus"
	"sync"
)

var wg sync.WaitGroup

// Go 框架处理协程,用于优雅启停
func Go(fun func(ctx context.Context) error, opts ...Option) error {
	wg.Add(1)
	options := &Options{}
	for _, o := range opts {
		o(options)
	}
	if options.ctx == nil {
		options.ctx = context.Background()
	}
	go func() {
		defer func() {
			if r := recover(); r != nil {
				logrus.Errorf("recovered in goroutine:%v", r)
			}
		}()
		defer wg.Done()
		_ = fun(options.ctx)
	}()
	return nil
}

// Wait 等待所有协程结束
func Wait() {
	wg.Wait()
}
