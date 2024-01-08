package gogo

import (
	"context"
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
		defer wg.Done()
		_ = fun(options.ctx)
	}()
	return nil
}

// Wait 等待所有协程结束
func Wait() {
	wg.Wait()
}
