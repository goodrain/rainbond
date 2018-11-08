package model

type Stream struct {
	Includes []string
}

func NewStream() *Stream {
	return &Stream{
		Includes: []string{
			"/export/servers/nginx/conf/servers-tcp.conf",
		},
	}
}
