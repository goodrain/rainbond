package zmq4

/*
#cgo !windows pkg-config: libzmq
#cgo windows CFLAGS: -I/usr/local/include
#cgo windows LDFLAGS: -L/usr/local/lib -lzmq
#include <zmq.h>
#if ZMQ_VERSION_MINOR < 2
#include <zmq_utils.h>
#endif
#include <stdlib.h>
#include <string.h>
#include "zmq4.h"

int
    zmq4_major = ZMQ_VERSION_MAJOR,
    zmq4_minor = ZMQ_VERSION_MINOR,
    zmq4_patch = ZMQ_VERSION_PATCH;

#if ZMQ_VERSION_MINOR > 0
// Version >= 4.1.x

typedef struct {
    uint16_t event;  // id of the event as bitfield
    int32_t  value;  // value is either error code, fd or reconnect interval
} zmq_event_t;

#else
// Version == 4.0.x

const char *zmq_msg_gets (zmq_msg_t *msg, const char *property) {
    return NULL;
}

int zmq_has (const char *capability) {
    return 0;
}

#if ZMQ_VERSION_PATCH < 5
// Version < 4.0.5

int zmq_proxy_steerable (const void *frontend, const void *backend, const void *capture, const void *control) {
    return -1;
}

#endif // Version < 4.0.5

#endif // Version == 4.0.x

void zmq4_get_event40(zmq_msg_t *msg, int *ev, int *val) {
    zmq_event_t event;
    const char* data = (char*)zmq_msg_data(msg);
    memcpy(&(event.event), data, sizeof(event.event));
    memcpy(&(event.value), data+sizeof(event.event), sizeof(event.value));
    *ev = (int)(event.event);
    *val = (int)(event.value);
}
void zmq4_get_event41(zmq_msg_t *msg, int *ev, int *val) {
    uint8_t *data = (uint8_t *) zmq_msg_data (msg);
    uint16_t event = *(uint16_t *) (data);
    *ev = (int)event;
    *val = (int)(*(uint32_t *) (data + 2));
}
void *zmq4_memcpy(void *dest, const void *src, size_t n) {
    return memcpy(dest, src, n);
}
*/
import "C"

import (
	"errors"
	"fmt"
	"runtime"
	"strings"
	"unsafe"
)

var (
	defaultCtx *Context

	major, minor, patch int

	ErrorContextClosed         = errors.New("Context is closed")
	ErrorSocketClosed          = errors.New("Socket is closed")
	ErrorMoreExpected          = errors.New("More expected")
	ErrorNotImplemented405     = errors.New("Not implemented, requires 0MQ version 4.0.5")
	ErrorNotImplemented41      = errors.New("Not implemented, requires 0MQ version 4.1")
	ErrorNotImplemented42      = errors.New("Not implemented, requires 0MQ version 4.2")
	ErrorNotImplementedWindows = errors.New("Not implemented on Windows")
	ErrorNoSocket              = errors.New("No such socket")
)

func init() {
	var err error
	defaultCtx = &Context{}
	defaultCtx.ctx, err = C.zmq_ctx_new()
	defaultCtx.opened = true
	if defaultCtx.ctx == nil {
		panic("Init of ZeroMQ context failed: " + errget(err).Error())
	}
	major, minor, patch = Version()
	if major != 4 {
		panic("Using zmq4 with ZeroMQ major version " + fmt.Sprint(major))
	}
	if major != int(C.zmq4_major) || minor != int(C.zmq4_minor) || patch != int(C.zmq4_patch) {
		panic(
			fmt.Sprintf(
				"zmq4 was installed with ZeroMQ version %d.%d.%d, but the application links with version %d.%d.%d",
				int(C.zmq4_major), int(C.zmq4_minor), int(C.zmq4_patch),
				major, minor, patch))
	}
}

//. Util

// Report 0MQ library version.
func Version() (major, minor, patch int) {
	var maj, min, pat C.int
	C.zmq_version(&maj, &min, &pat)
	return int(maj), int(min), int(pat)
}

// Get 0MQ error message string.
func Error(e int) string {
	return C.GoString(C.zmq_strerror(C.int(e)))
}

//. Context

const (
	MaxSocketsDflt = int(C.ZMQ_MAX_SOCKETS_DFLT)
	IoThreadsDflt  = int(C.ZMQ_IO_THREADS_DFLT)
)

/*
A context that is not the default context.
*/
type Context struct {
	ctx    unsafe.Pointer
	opened bool
	err    error
}

// Create a new context.
func NewContext() (ctx *Context, err error) {
	ctx = &Context{}
	c, e := C.zmq_ctx_new()
	if c == nil {
		err = errget(e)
		ctx.err = err
	} else {
		ctx.ctx = c
		ctx.opened = true
		runtime.SetFinalizer(ctx, (*Context).Term)
	}
	return
}

/*
Terminates the default context.

For linger behavior, see: http://api.zeromq.org/4-1:zmq-ctx-term
*/
func Term() error {
	return defaultCtx.Term()
}

/*
Terminates the context.

For linger behavior, see: http://api.zeromq.org/4-1:zmq-ctx-term
*/
func (ctx *Context) Term() error {
	if ctx.opened {
		ctx.opened = false
		n, err := C.zmq_ctx_term(ctx.ctx)
		if n != 0 {
			ctx.err = errget(err)
		}
	}
	return ctx.err
}

func getOption(ctx *Context, o C.int) (int, error) {
	if !ctx.opened {
		return 0, ErrorContextClosed
	}
	nc, err := C.zmq_ctx_get(ctx.ctx, o)
	n := int(nc)
	if n < 0 {
		return n, errget(err)
	}
	return n, nil
}

// Returns the size of the 0MQ thread pool in the default context.
func GetIoThreads() (int, error) {
	return defaultCtx.GetIoThreads()
}

// Returns the size of the 0MQ thread pool.
func (ctx *Context) GetIoThreads() (int, error) {
	return getOption(ctx, C.ZMQ_IO_THREADS)
}

// Returns the maximum number of sockets allowed in the default context.
func GetMaxSockets() (int, error) {
	return defaultCtx.GetMaxSockets()
}

// Returns the maximum number of sockets allowed.
func (ctx *Context) GetMaxSockets() (int, error) {
	return getOption(ctx, C.ZMQ_MAX_SOCKETS)
}

/*
Returns the maximum message size in the default context.

Returns ErrorNotImplemented42 with ZeroMQ version < 4.2
*/
func GetMaxMsgsz() (int, error) {
	return defaultCtx.GetMaxMsgsz()
}

/*
Returns the maximum message size.

Returns ErrorNotImplemented42 with ZeroMQ version < 4.2
*/
func (ctx *Context) GetMaxMsgsz() (int, error) {
	if minor < 2 {
		return 0, ErrorNotImplemented42
	}
	return getOption(ctx, C.ZMQ_MAX_MSGSZ)
}

// Returns the IPv6 option in the default context.
func GetIpv6() (bool, error) {
	return defaultCtx.GetIpv6()
}

// Returns the IPv6 option.
func (ctx *Context) GetIpv6() (bool, error) {
	i, e := getOption(ctx, C.ZMQ_IPV6)
	if i == 0 {
		return false, e
	}
	return true, e
}

/*
Returns the blocky setting in the default context.

Returns ErrorNotImplemented42 with ZeroMQ version < 4.2
*/
func GetBlocky() (bool, error) {
	return defaultCtx.GetBlocky()
}

/*
Returns the blocky setting.

Returns ErrorNotImplemented42 with ZeroMQ version < 4.2
*/
func (ctx *Context) GetBlocky() (bool, error) {
	if minor < 2 {
		return false, ErrorNotImplemented42
	}
	i, e := getOption(ctx, C.ZMQ_BLOCKY)
	if i == 0 {
		return false, e
	}
	return true, e
}

func setOption(ctx *Context, o C.int, n int) error {
	if !ctx.opened {
		return ErrorContextClosed
	}
	i, err := C.zmq_ctx_set(ctx.ctx, o, C.int(n))
	if int(i) != 0 {
		return errget(err)
	}
	return nil
}

/*
Specifies the size of the 0MQ thread pool to handle I/O operations in
the default context. If your application is using only the inproc
transport for messaging you may set this to zero, otherwise set it to at
least one. This option only applies before creating any sockets.

Default value: 1
*/
func SetIoThreads(n int) error {
	return defaultCtx.SetIoThreads(n)
}

/*
Specifies the size of the 0MQ thread pool to handle I/O operations. If
your application is using only the inproc transport for messaging you
may set this to zero, otherwise set it to at least one. This option only
applies before creating any sockets.

Default value: 1
*/
func (ctx *Context) SetIoThreads(n int) error {
	return setOption(ctx, C.ZMQ_IO_THREADS, n)
}

/*
Sets the scheduling policy for default context’s thread pool.

This option requires ZeroMQ version 4.1, and is not available on Windows.

Supported values for this option can be found in sched.h file, or at
http://man7.org/linux/man-pages/man2/sched_setscheduler.2.html

This option only applies before creating any sockets on the context.

Default value: -1

Returns ErrorNotImplemented41 with ZeroMQ version < 4.1

Returns ErrorNotImplementedWindows on Windows
*/
func SetThreadSchedPolicy(n int) error {
	return defaultCtx.SetThreadSchedPolicy(n)
}

/*
Sets scheduling priority for default context’s thread pool.

This option requires ZeroMQ version 4.1, and is not available on Windows.

Supported values for this option depend on chosen scheduling policy.
Details can be found in sched.h file, or at
http://man7.org/linux/man-pages/man2/sched_setscheduler.2.html

This option only applies before creating any sockets on the context.

Default value: -1

Returns ErrorNotImplemented41 with ZeroMQ version < 4.1

Returns ErrorNotImplementedWindows on Windows
*/
func SetThreadPriority(n int) error {
	return defaultCtx.SetThreadPriority(n)
}

/*
Set maximum message size in the default context.

Default value: INT_MAX

Returns ErrorNotImplemented42 with ZeroMQ version < 4.2
*/
func SetMaxMsgsz(n int) error {
	return defaultCtx.SetMaxMsgsz(n)
}

/*
Set maximum message size.

Default value: INT_MAX

Returns ErrorNotImplemented42 with ZeroMQ version < 4.2
*/
func (ctx *Context) SetMaxMsgsz(n int) error {
	if minor < 2 {
		return ErrorNotImplemented42
	}
	return setOption(ctx, C.ZMQ_MAX_MSGSZ, n)
}

/*
Sets the maximum number of sockets allowed in the default context.

Default value: 1024
*/
func SetMaxSockets(n int) error {
	return defaultCtx.SetMaxSockets(n)
}

/*
Sets the maximum number of sockets allowed.

Default value: 1024
*/
func (ctx *Context) SetMaxSockets(n int) error {
	return setOption(ctx, C.ZMQ_MAX_SOCKETS, n)
}

/*
Sets the IPv6 value for all sockets created in the default context from this point onwards.
A value of true means IPv6 is enabled, while false means the socket will use only IPv4.
When IPv6 is enabled, a socket will connect to, or accept connections from, both IPv4 and IPv6 hosts.

Default value: false
*/
func SetIpv6(i bool) error {
	return defaultCtx.SetIpv6(i)
}

/*
Sets the IPv6 value for all sockets created in the context from this point onwards.
A value of true means IPv6 is enabled, while false means the socket will use only IPv4.
When IPv6 is enabled, a socket will connect to, or accept connections from, both IPv4 and IPv6 hosts.

Default value: false
*/
func (ctx *Context) SetIpv6(i bool) error {
	n := 0
	if i {
		n = 1
	}
	return setOption(ctx, C.ZMQ_IPV6, n)
}

/*
Sets the blocky behavior in the default context.

See: http://api.zeromq.org/4-2:zmq-ctx-set#toc3

Returns ErrorNotImplemented42 with ZeroMQ version < 4.2
*/
func SetBlocky(i bool) error {
	return defaultCtx.SetBlocky(i)
}

/*
Sets the blocky behavior.

See: http://api.zeromq.org/4-2:zmq-ctx-set#toc3

Returns ErrorNotImplemented42 with ZeroMQ version < 4.2
*/
func (ctx *Context) SetBlocky(i bool) error {
	if minor < 2 {
		return ErrorNotImplemented42
	}
	n := 0
	if i {
		n = 1
	}
	return setOption(ctx, C.ZMQ_BLOCKY, n)
}

//. Sockets

// Specifies the type of a socket, used by NewSocket()
type Type int

const (
	// Constants for NewSocket()
	// See: http://api.zeromq.org/4-1:zmq-socket#toc3
	REQ    = Type(C.ZMQ_REQ)
	REP    = Type(C.ZMQ_REP)
	DEALER = Type(C.ZMQ_DEALER)
	ROUTER = Type(C.ZMQ_ROUTER)
	PUB    = Type(C.ZMQ_PUB)
	SUB    = Type(C.ZMQ_SUB)
	XPUB   = Type(C.ZMQ_XPUB)
	XSUB   = Type(C.ZMQ_XSUB)
	PUSH   = Type(C.ZMQ_PUSH)
	PULL   = Type(C.ZMQ_PULL)
	PAIR   = Type(C.ZMQ_PAIR)
	STREAM = Type(C.ZMQ_STREAM)
)

/*
Socket type as string.
*/
func (t Type) String() string {
	switch t {
	case REQ:
		return "REQ"
	case REP:
		return "REP"
	case DEALER:
		return "DEALER"
	case ROUTER:
		return "ROUTER"
	case PUB:
		return "PUB"
	case SUB:
		return "SUB"
	case XPUB:
		return "XPUB"
	case XSUB:
		return "XSUB"
	case PUSH:
		return "PUSH"
	case PULL:
		return "PULL"
	case PAIR:
		return "PAIR"
	case STREAM:
		return "STREAM"
	}
	return "<INVALID>"
}

// Used by  (*Socket)Send() and (*Socket)Recv()
type Flag int

const (
	// Flags for (*Socket)Send(), (*Socket)Recv()
	// For Send, see: http://api.zeromq.org/4-1:zmq-send#toc2
	// For Recv, see: http://api.zeromq.org/4-1:zmq-msg-recv#toc2
	DONTWAIT = Flag(C.ZMQ_DONTWAIT)
	SNDMORE  = Flag(C.ZMQ_SNDMORE)
)

/*
Socket flag as string.
*/
func (f Flag) String() string {
	ff := make([]string, 0)
	if f&DONTWAIT != 0 {
		ff = append(ff, "DONTWAIT")
	}
	if f&SNDMORE != 0 {
		ff = append(ff, "SNDMORE")
	}
	if len(ff) == 0 {
		return "<NONE>"
	}
	return strings.Join(ff, "|")
}

// Used by (*Socket)Monitor() and (*Socket)RecvEvent()
type Event int

const (
	// Flags for (*Socket)Monitor() and (*Socket)RecvEvent()
	// See: http://api.zeromq.org/4-1:zmq-socket-monitor#toc3
	EVENT_ALL             = Event(C.ZMQ_EVENT_ALL)
	EVENT_CONNECTED       = Event(C.ZMQ_EVENT_CONNECTED)
	EVENT_CONNECT_DELAYED = Event(C.ZMQ_EVENT_CONNECT_DELAYED)
	EVENT_CONNECT_RETRIED = Event(C.ZMQ_EVENT_CONNECT_RETRIED)
	EVENT_LISTENING       = Event(C.ZMQ_EVENT_LISTENING)
	EVENT_BIND_FAILED     = Event(C.ZMQ_EVENT_BIND_FAILED)
	EVENT_ACCEPTED        = Event(C.ZMQ_EVENT_ACCEPTED)
	EVENT_ACCEPT_FAILED   = Event(C.ZMQ_EVENT_ACCEPT_FAILED)
	EVENT_CLOSED          = Event(C.ZMQ_EVENT_CLOSED)
	EVENT_CLOSE_FAILED    = Event(C.ZMQ_EVENT_CLOSE_FAILED)
	EVENT_DISCONNECTED    = Event(C.ZMQ_EVENT_DISCONNECTED)
	EVENT_MONITOR_STOPPED = Event(C.ZMQ_EVENT_MONITOR_STOPPED)
)

/*
Socket event as string.
*/
func (e Event) String() string {
	if e == EVENT_ALL {
		return "EVENT_ALL"
	}
	ee := make([]string, 0)
	if e&EVENT_CONNECTED != 0 {
		ee = append(ee, "EVENT_CONNECTED")
	}
	if e&EVENT_CONNECT_DELAYED != 0 {
		ee = append(ee, "EVENT_CONNECT_DELAYED")
	}
	if e&EVENT_CONNECT_RETRIED != 0 {
		ee = append(ee, "EVENT_CONNECT_RETRIED")
	}
	if e&EVENT_LISTENING != 0 {
		ee = append(ee, "EVENT_LISTENING")
	}
	if e&EVENT_BIND_FAILED != 0 {
		ee = append(ee, "EVENT_BIND_FAILED")
	}
	if e&EVENT_ACCEPTED != 0 {
		ee = append(ee, "EVENT_ACCEPTED")
	}
	if e&EVENT_ACCEPT_FAILED != 0 {
		ee = append(ee, "EVENT_ACCEPT_FAILED")
	}
	if e&EVENT_CLOSED != 0 {
		ee = append(ee, "EVENT_CLOSED")
	}
	if e&EVENT_CLOSE_FAILED != 0 {
		ee = append(ee, "EVENT_CLOSE_FAILED")
	}
	if e&EVENT_DISCONNECTED != 0 {
		ee = append(ee, "EVENT_DISCONNECTED")
	}
	if len(ee) == 0 {
		return "<NONE>"
	}
	return strings.Join(ee, "|")
}

// Used by (soc *Socket)GetEvents()
type State int

const (
	// Flags for (*Socket)GetEvents()
	// See: http://api.zeromq.org/4-1:zmq-getsockopt#toc8
	POLLIN  = State(C.ZMQ_POLLIN)
	POLLOUT = State(C.ZMQ_POLLOUT)
)

/*
Socket state as string.
*/
func (s State) String() string {
	ss := make([]string, 0)
	if s&POLLIN != 0 {
		ss = append(ss, "POLLIN")
	}
	if s&POLLOUT != 0 {
		ss = append(ss, "POLLOUT")
	}
	if len(ss) == 0 {
		return "<NONE>"
	}
	return strings.Join(ss, "|")
}

// Specifies the security mechanism, used by (*Socket)GetMechanism()
type Mechanism int

const (
	// Constants for (*Socket)GetMechanism()
	// See: http://api.zeromq.org/4-1:zmq-getsockopt#toc22
	NULL   = Mechanism(C.ZMQ_NULL)
	PLAIN  = Mechanism(C.ZMQ_PLAIN)
	CURVE  = Mechanism(C.ZMQ_CURVE)
	GSSAPI = Mechanism(C.ZMQ_GSSAPI)
)

/*
Security mechanism as string.
*/
func (m Mechanism) String() string {
	switch m {
	case NULL:
		return "NULL"
	case PLAIN:
		return "PLAIN"
	case CURVE:
		return "CURVE"
	case GSSAPI:
		return "GSSAPI"
	}
	return "<INVALID>"
}

/*
Socket functions starting with `Set` or `Get` are used for setting and
getting socket options.
*/
type Socket struct {
	soc    unsafe.Pointer
	ctx    *Context
	opened bool
	err    error
}

/*
Socket as string.
*/
func (soc Socket) String() string {
	if !soc.opened {
		return "Socket(CLOSED)"
	}
	t, err := soc.GetType()
	if err != nil {
		return fmt.Sprintf("Socket(%v)", err)
	}
	i, err := soc.GetIdentity()
	if err == nil && i != "" {
		return fmt.Sprintf("Socket(%v,%q)", t, i)
	}
	return fmt.Sprintf("Socket(%v,%p)", t, soc.soc)
}

/*
Create 0MQ socket in the default context.

WARNING:
The Socket is not thread safe. This means that you cannot access the same Socket
from different goroutines without using something like a mutex.

For a description of socket types, see: http://api.zeromq.org/4-1:zmq-socket#toc3
*/
func NewSocket(t Type) (soc *Socket, err error) {
	return defaultCtx.NewSocket(t)
}

/*
Create 0MQ socket in the given context.

WARNING:
The Socket is not thread safe. This means that you cannot access the same Socket
from different goroutines without using something like a mutex.

For a description of socket types, see: http://api.zeromq.org/4-1:zmq-socket#toc3
*/
func (ctx *Context) NewSocket(t Type) (soc *Socket, err error) {
	soc = &Socket{}
	if !ctx.opened {
		return soc, ErrorContextClosed
	}
	s, e := C.zmq_socket(ctx.ctx, C.int(t))
	if s == nil {
		err = errget(e)
		soc.err = err
	} else {
		soc.soc = s
		soc.ctx = ctx
		soc.opened = true
		runtime.SetFinalizer(soc, (*Socket).Close)
	}
	return
}

// If not called explicitly, the socket will be closed on garbage collection
func (soc *Socket) Close() error {
	if soc.opened {
		soc.opened = false
		if i, err := C.zmq_close(soc.soc); int(i) != 0 {
			soc.err = errget(err)
		}
		soc.soc = unsafe.Pointer(nil)
		soc.ctx = nil
	}
	return soc.err
}

// Return the context associated with a socket
func (soc *Socket) Context() (*Context, error) {
	if !soc.opened {
		return nil, ErrorSocketClosed
	}
	return soc.ctx, nil
}

/*
Accept incoming connections on a socket.

For a description of endpoint, see: http://api.zeromq.org/4-1:zmq-bind#toc2
*/
func (soc *Socket) Bind(endpoint string) error {
	if !soc.opened {
		return ErrorSocketClosed
	}
	s := C.CString(endpoint)
	defer C.free(unsafe.Pointer(s))
	if i, err := C.zmq_bind(soc.soc, s); int(i) != 0 {
		return errget(err)
	}
	return nil
}

/*
Stop accepting connections on a socket.

For a description of endpoint, see: http://api.zeromq.org/4-1:zmq-bind#toc2
*/
func (soc *Socket) Unbind(endpoint string) error {
	if !soc.opened {
		return ErrorSocketClosed
	}
	s := C.CString(endpoint)
	defer C.free(unsafe.Pointer(s))
	if i, err := C.zmq_unbind(soc.soc, s); int(i) != 0 {
		return errget(err)
	}
	return nil
}

/*
Create outgoing connection from socket.

For a description of endpoint, see: http://api.zeromq.org/4-1:zmq-connect#toc2
*/
func (soc *Socket) Connect(endpoint string) error {
	if !soc.opened {
		return ErrorSocketClosed
	}
	s := C.CString(endpoint)
	defer C.free(unsafe.Pointer(s))
	if i, err := C.zmq_connect(soc.soc, s); int(i) != 0 {
		return errget(err)
	}
	return nil
}

/*
Disconnect a socket.

For a description of endpoint, see: http://api.zeromq.org/4-1:zmq-disconnect#toc2
*/
func (soc *Socket) Disconnect(endpoint string) error {
	if !soc.opened {
		return ErrorSocketClosed
	}
	s := C.CString(endpoint)
	defer C.free(unsafe.Pointer(s))
	if i, err := C.zmq_disconnect(soc.soc, s); int(i) != 0 {
		return errget(err)
	}
	return nil
}

/*
Receive a message part from a socket.

For a description of flags, see: http://api.zeromq.org/4-1:zmq-msg-recv#toc2
*/
func (soc *Socket) Recv(flags Flag) (string, error) {
	b, err := soc.RecvBytes(flags)
	return string(b), err
}

/*
Receive a message part from a socket.

For a description of flags, see: http://api.zeromq.org/4-1:zmq-msg-recv#toc2
*/
func (soc *Socket) RecvBytes(flags Flag) ([]byte, error) {
	if !soc.opened {
		return []byte{}, ErrorSocketClosed
	}
	var msg C.zmq_msg_t
	if i, err := C.zmq_msg_init(&msg); i != 0 {
		return []byte{}, errget(err)
	}
	defer C.zmq_msg_close(&msg)

	size, err := C.zmq_msg_recv(&msg, soc.soc, C.int(flags))
	if size < 0 {
		return []byte{}, errget(err)
	}
	if size == 0 {
		return []byte{}, nil
	}
	data := make([]byte, int(size))
	C.zmq4_memcpy(unsafe.Pointer(&data[0]), C.zmq_msg_data(&msg), C.size_t(size))
	return data, nil
}

/*
Send a message part on a socket.

For a description of flags, see: http://api.zeromq.org/4-1:zmq-send#toc2
*/
func (soc *Socket) Send(data string, flags Flag) (int, error) {
	return soc.SendBytes([]byte(data), flags)
}

/*
Send a message part on a socket.

For a description of flags, see: http://api.zeromq.org/4-1:zmq-send#toc2
*/
func (soc *Socket) SendBytes(data []byte, flags Flag) (int, error) {
	if !soc.opened {
		return 0, ErrorSocketClosed
	}
	d := data
	if len(data) == 0 {
		d = []byte{0}
	}
	size, err := C.zmq_send(soc.soc, unsafe.Pointer(&d[0]), C.size_t(len(data)), C.int(flags))
	if size < 0 {
		return int(size), errget(err)
	}
	return int(size), nil
}

/*
Register a monitoring callback.

See: http://api.zeromq.org/4-1:zmq-socket-monitor#toc2

WARNING: Closing a context with a monitoring callback will lead to random crashes.
This is a bug in the ZeroMQ library.
The monitoring callback has the same context as the socket it was created for.

Example:

    package main

    import (
        zmq "github.com/pebbe/zmq4"
        "log"
        "time"
    )

    func rep_socket_monitor(addr string) {
        s, err := zmq.NewSocket(zmq.PAIR)
        if err != nil {
            log.Fatalln(err)
        }
        err = s.Connect(addr)
        if err != nil {
            log.Fatalln(err)
        }
        for {
            a, b, c, err := s.RecvEvent(0)
            if err != nil {
                log.Println(err)
                break
            }
            log.Println(a, b, c)
        }
        s.Close()
    }

    func main() {

        // REP socket
        rep, err := zmq.NewSocket(zmq.REP)
        if err != nil {
            log.Fatalln(err)
        }

        // REP socket monitor, all events
        err = rep.Monitor("inproc://monitor.rep", zmq.EVENT_ALL)
        if err != nil {
            log.Fatalln(err)
        }
        go rep_socket_monitor("inproc://monitor.rep")

        // Generate an event
        rep.Bind("tcp://*:5555")
        if err != nil {
            log.Fatalln(err)
        }

        // Allow some time for event detection
        time.Sleep(time.Second)

        rep.Close()
        zmq.Term()
    }
*/
func (soc *Socket) Monitor(addr string, events Event) error {
	if !soc.opened {
		return ErrorSocketClosed
	}
	if addr == "" {
		if i, err := C.zmq_socket_monitor(soc.soc, nil, C.int(events)); i != 0 {
			return errget(err)
		}
		return nil
	}

	s := C.CString(addr)
	defer C.free(unsafe.Pointer(s))
	if i, err := C.zmq_socket_monitor(soc.soc, s, C.int(events)); i != 0 {
		return errget(err)
	}
	return nil
}

/*
Receive a message part from a socket interpreted as an event.

For a description of flags, see: http://api.zeromq.org/4-1:zmq-msg-recv#toc2

For a description of event_type, see: http://api.zeromq.org/4-1:zmq-socket-monitor#toc3

For an example, see: func (*Socket) Monitor
*/
func (soc *Socket) RecvEvent(flags Flag) (event_type Event, addr string, value int, err error) {
	if !soc.opened {
		return EVENT_ALL, "", 0, ErrorSocketClosed
	}
	var msg C.zmq_msg_t
	if i, e := C.zmq_msg_init(&msg); i != 0 {
		err = errget(e)
		return
	}
	defer C.zmq_msg_close(&msg)
	size, e := C.zmq_msg_recv(&msg, soc.soc, C.int(flags))
	if size < 0 {
		err = errget(e)
		return
	}
	et := C.int(0)
	val := C.int(0)

	if minor == 0 {
		C.zmq4_get_event40(&msg, &et, &val)
	} else {
		C.zmq4_get_event41(&msg, &et, &val)
	}
	more, e := soc.GetRcvmore()
	if e != nil {
		err = errget(e)
		return
	}
	if !more {
		err = ErrorMoreExpected
		return
	}
	addr, e = soc.Recv(flags)
	if e != nil {
		err = errget(e)
		return
	}

	event_type = Event(et)
	value = int(val)

	return
}

/*
Start built-in ØMQ proxy

See: http://api.zeromq.org/4-1:zmq-proxy#toc2
*/
func Proxy(frontend, backend, capture *Socket) error {
	if !(frontend.opened && backend.opened && (capture == nil || capture.opened)) {
		return ErrorSocketClosed
	}
	var capt unsafe.Pointer
	if capture != nil {
		capt = capture.soc
	}
	_, err := C.zmq_proxy(frontend.soc, backend.soc, capt)
	return errget(err)
}

/*
Start built-in ØMQ proxy with PAUSE/RESUME/TERMINATE control flow

Returns ErrorNotImplemented405 with ZeroMQ version < 4.0.5

See: http://api.zeromq.org/4-1:zmq-proxy-steerable#toc2
*/
func ProxySteerable(frontend, backend, capture, control *Socket) error {
	if minor == 0 && patch < 5 {
		return ErrorNotImplemented405
	}
	if !(frontend.opened && backend.opened && (capture == nil || capture.opened) && (control == nil || control.opened)) {
		return ErrorSocketClosed
	}
	var capt, ctrl unsafe.Pointer
	if capture != nil {
		capt = capture.soc
	}
	if control != nil {
		ctrl = control.soc
	}
	i, err := C.zmq_proxy_steerable(frontend.soc, backend.soc, capt, ctrl)
	if i < 0 {
		return errget(err)
	}
	return nil
}

//. CURVE

/*
Encode a binary key as Z85 printable text

See: http://api.zeromq.org/4-1:zmq-z85-encode
*/
func Z85encode(data string) string {
	l1 := len(data)
	if l1%4 != 0 {
		panic("Z85encode: Length of data not a multiple of 4")
	}
	d := []byte(data)

	l2 := 5 * l1 / 4
	dest := make([]byte, l2+1)

	C.zmq_z85_encode((*C.char)(unsafe.Pointer(&dest[0])), (*C.uint8_t)(&d[0]), C.size_t(l1))

	return string(dest[:l2])
}

/*
Decode a binary key from Z85 printable text

See: http://api.zeromq.org/4-1:zmq-z85-decode
*/
func Z85decode(s string) string {
	l1 := len(s)
	if l1%5 != 0 {
		panic("Z85decode: Length of Z85 string not a multiple of 5")
	}
	l2 := 4 * l1 / 5
	dest := make([]byte, l2)
	cs := C.CString(s)
	defer C.free(unsafe.Pointer(cs))
	C.zmq_z85_decode((*C.uint8_t)(&dest[0]), cs)
	return string(dest)
}

/*
Generate a new CURVE keypair

See: http://api.zeromq.org/4-1:zmq-curve-keypair#toc2
*/
func NewCurveKeypair() (z85_public_key, z85_secret_key string, err error) {
	var pubkey, seckey [41]byte
	if i, err := C.zmq_curve_keypair((*C.char)(unsafe.Pointer(&pubkey[0])), (*C.char)(unsafe.Pointer(&seckey[0]))); i != 0 {
		return "", "", errget(err)
	}
	return string(pubkey[:40]), string(seckey[:40]), nil
}

/*
Receive a message part with metadata.

This requires ZeroMQ version 4.1.0. Lower versions will return the message part without metadata.

The returned metadata map contains only those properties that exist on the message.

For a description of flags, see: http://api.zeromq.org/4-1:zmq-msg-recv#toc2

For a description of metadata, see: http://api.zeromq.org/4-1:zmq-msg-gets#toc3
*/
func (soc *Socket) RecvWithMetadata(flags Flag, properties ...string) (msg string, metadata map[string]string, err error) {
	b, p, err := soc.RecvBytesWithMetadata(flags, properties...)
	return string(b), p, err
}

/*
Receive a message part with metadata.

This requires ZeroMQ version 4.1.0. Lower versions will return the message part without metadata.

The returned metadata map contains only those properties that exist on the message.

For a description of flags, see: http://api.zeromq.org/4-1:zmq-msg-recv#toc2

For a description of metadata, see: http://api.zeromq.org/4-1:zmq-msg-gets#toc3
*/
func (soc *Socket) RecvBytesWithMetadata(flags Flag, properties ...string) (msg []byte, metadata map[string]string, err error) {
	if !soc.opened {
		return []byte{}, map[string]string{}, ErrorSocketClosed
	}

	metadata = make(map[string]string)

	var m C.zmq_msg_t
	if i, err := C.zmq_msg_init(&m); i != 0 {
		return []byte{}, metadata, errget(err)
	}
	defer C.zmq_msg_close(&m)

	size, err := C.zmq_msg_recv(&m, soc.soc, C.int(flags))
	if size < 0 {
		return []byte{}, metadata, errget(err)
	}

	data := make([]byte, int(size))
	if size > 0 {
		C.zmq4_memcpy(unsafe.Pointer(&data[0]), C.zmq_msg_data(&m), C.size_t(size))
	}

	if minor > 0 {
		for _, p := range properties {
			ps := C.CString(p)
			s, err := C.zmq_msg_gets(&m, ps)
			if err == nil {
				metadata[p] = C.GoString(s)
			}
			C.free(unsafe.Pointer(ps))
		}
	}
	return data, metadata, nil
}

func hasCap(s string) (value bool) {
	if minor < 1 {
		return false
	}
	cs := C.CString(s)
	defer C.free(unsafe.Pointer(cs))
	return C.zmq_has(cs) != 0
}

// Returns false for ZeroMQ version < 4.1.0
//
// Else: returns true if the library supports the ipc:// protocol
func HasIpc() bool {
	return hasCap("ipc")
}

// Returns false for ZeroMQ version < 4.1.0
//
// Else: returns true if the library supports the pgm:// protocol
func HasPgm() bool {
	return hasCap("pgm")
}

// Returns false for ZeroMQ version < 4.1.0
//
// Else: returns true if the library supports the tipc:// protocol
func HasTipc() bool {
	return hasCap("tipc")
}

// Returns false for ZeroMQ version < 4.1.0
//
// Else: returns true if the library supports the norm:// protocol
func HasNorm() bool {
	return hasCap("norm")
}

// Returns false for ZeroMQ version < 4.1.0
//
// Else: returns true if the library supports the CURVE security mechanism
func HasCurve() bool {
	return hasCap("curve")
}

// Returns false for ZeroMQ version < 4.1.0
//
// Else: returns true if the library supports the GSSAPI security mechanism
func HasGssapi() bool {
	return hasCap("gssapi")
}
