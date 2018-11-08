package model

type Nginx struct {
	WorkerProcesses int
	EventLog        EventLog
	Events          Events
	Includes        []string
}

type EventLog struct {
	Path  string
	Level string
}

type Events struct {
	WorkerConnections int
}

func NewNginx() *Nginx {
	return &Nginx{
		WorkerProcesses: 2, // TODO
		Includes: []string{
			"/export/servers/nginx/conf/http.conf",
		},
	}
}
