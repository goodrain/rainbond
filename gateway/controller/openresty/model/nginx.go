package model

import (
	"os/user"
	"runtime"

	"github.com/goodrain/rainbond/cmd/gateway/option"
)

//Nginx nginx config model
type Nginx struct {
	WorkerProcesses    int
	WorkerRlimitNofile int
	ErrorLog           string
	User               string
	EventLog           EventLog
	Events             Events
	HTTP               *HTTP
}

// EventLog -
type EventLog struct {
	Path  string
	Level string
}

//Events nginx events config model
type Events struct {
	WorkerConnections int
	EnableEpoll       bool
	EnableMultiAccept bool
}

//NewNginx new nginx config
func NewNginx(conf option.Config) *Nginx {
	if conf.NginxUser != "" {
		if u, err := user.Current(); err == nil {
			if conf.NginxUser == u.Username {
				//if set user name like run user,do not set
				conf.NginxUser = ""
			}
		}
	}
	if conf.WorkerProcesses == 0 {
		conf.WorkerProcesses = runtime.NumCPU()
	}
	return &Nginx{
		WorkerProcesses:    conf.WorkerProcesses,
		WorkerRlimitNofile: conf.WorkerRlimitNofile,
		User:               conf.NginxUser,
		ErrorLog:           conf.ErrorLog,
		Events: Events{
			WorkerConnections: conf.WorkerConnections,
			EnableEpoll:       conf.EnableEpool,
			EnableMultiAccept: conf.EnableMultiAccept,
		},
	}
}
