package controller

import (
	"github.com/goodrain/rainbond/gateway/v1"
)

type GWServicer interface {
	Start(errCh chan error)
	PersistConfig(conf *v1.Config) error
	UpdatePools(pools []*v1.Pool) error
	WaitPluginReady()
	Stop() error
}
