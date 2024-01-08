package gogo

import "context"

type Option func(*Options)

type Options struct {
	ctx context.Context
}

func WithContext(ctx context.Context) Option {
	return func(o *Options) {
		o.ctx = ctx
	}
}
