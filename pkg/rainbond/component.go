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
