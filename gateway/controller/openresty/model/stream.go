package model

// Stream -
type Stream struct {
	Includes []string
}

// NewStream creates a new stream.
func NewStream() *Stream {
	return &Stream{
		Includes: []string{
			"/export/servers/nginx/conf/servers-tcp.conf",
		},
	}
}
