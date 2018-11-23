package controller

import (
	"github.com/goodrain/rainbond/gateway/v1"
)

type GWServicer interface {
	Start() error
	Starts(errCh chan error)
	PersistConfig(conf *v1.Config) error
	UpdatePools(pools []*v1.Pool) error
	DeletePools(pools []*v1.Pool) error
	WaitPluginReady()
	Stop() error
}
