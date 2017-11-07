// +build !windows

package zmq4

/*
#include <zmq.h>
*/
import "C"

// ZMQ_FD: Retrieve file descriptor associated with the socket
//
// See: http://api.zeromq.org/4-1:zmq-getsockopt#toc9
func (soc *Socket) GetFd() (int, error) {
	return soc.getInt(C.ZMQ_FD)
}
