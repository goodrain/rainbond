package zmq4

/*
#include <zmq.h>
*/
import "C"

import (
	"syscall"
)

// An Errno is an unsigned number describing an error condition as returned by a call to ZeroMQ.
// It implements the error interface.
// The number is either a standard system error, or an error defined by the C library of ZeroMQ.
type Errno uintptr

const (
	// Error conditions defined by the C library of ZeroMQ.

	// On Windows platform some of the standard POSIX errnos are not defined.
	EADDRINUSE      = Errno(C.EADDRINUSE)
	EADDRNOTAVAIL   = Errno(C.EADDRNOTAVAIL)
	EAFNOSUPPORT    = Errno(C.EAFNOSUPPORT)
	ECONNABORTED    = Errno(C.ECONNABORTED)
	ECONNREFUSED    = Errno(C.ECONNREFUSED)
	ECONNRESET      = Errno(C.ECONNRESET)
	EHOSTUNREACH    = Errno(C.EHOSTUNREACH)
	EINPROGRESS     = Errno(C.EINPROGRESS)
	EMSGSIZE        = Errno(C.EMSGSIZE)
	ENETDOWN        = Errno(C.ENETDOWN)
	ENETRESET       = Errno(C.ENETRESET)
	ENETUNREACH     = Errno(C.ENETUNREACH)
	ENOBUFS         = Errno(C.ENOBUFS)
	ENOTCONN        = Errno(C.ENOTCONN)
	ENOTSOCK        = Errno(C.ENOTSOCK)
	ENOTSUP         = Errno(C.ENOTSUP)
	EPROTONOSUPPORT = Errno(C.EPROTONOSUPPORT)
	ETIMEDOUT       = Errno(C.ETIMEDOUT)

	// Native 0MQ error codes.
	EFSM           = Errno(C.EFSM)
	EMTHREAD       = Errno(C.EMTHREAD)
	ENOCOMPATPROTO = Errno(C.ENOCOMPATPROTO)
	ETERM          = Errno(C.ETERM)
)

func errget(err error) error {
	eno, ok := err.(syscall.Errno)
	if ok {
		return Errno(eno)
	}
	return err
}

// Return Errno as string.
func (errno Errno) Error() string {
	if errno >= C.ZMQ_HAUSNUMERO {
		return C.GoString(C.zmq_strerror(C.int(errno)))
	}
	return syscall.Errno(errno).Error()
}

/*
Convert error to Errno.

Example usage:

    switch AsErrno(err) {

    case zmq.Errno(syscall.EINTR):
        // standard system error

        // call was interrupted

    case zmq.ETERM:
        // error defined by ZeroMQ

        // context was terminated

    }

See also: examples/interrupt.go
*/
func AsErrno(err error) Errno {
	if eno, ok := err.(Errno); ok {
		return eno
	}
	if eno, ok := err.(syscall.Errno); ok {
		return Errno(eno)
	}
	return Errno(0)
}
