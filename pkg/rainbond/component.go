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

package rainbond

import (
	"context"
	"github.com/goodrain/rainbond/config/configs"
)

type Component interface {
	Start(ctx context.Context, cfg *configs.Config) error
	CloseHandle()
}

type ComponentCancel interface {
	Component
	StartCancel(ctx context.Context, cancel context.CancelFunc, cfg *configs.Config) error
}

type FuncComponent func(ctx context.Context, cfg *configs.Config) error

func (f FuncComponent) Start(ctx context.Context, cfg *configs.Config) error {
	return f(ctx, cfg)
}

func (f FuncComponent) CloseHandle() {
}

type FuncComponentCancel func(ctx context.Context, cancel context.CancelFunc, cfg *configs.Config) error

func (f FuncComponentCancel) Start(ctx context.Context, cfg *configs.Config) error {
	return f.StartCancel(ctx, nil, cfg)
}

func (f FuncComponentCancel) StartCancel(ctx context.Context, cancel context.CancelFunc, cfg *configs.Config) error {
	return f(ctx, cancel, cfg)
}

func (f FuncComponentCancel) CloseHandle() {
}
