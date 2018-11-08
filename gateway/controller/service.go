package controller

import (
	"github.com/goodrain/rainbond/gateway/v1"
)

type GWServicer interface {
	Start() error
	PersistConfig(conf *v1.Config) error
}
