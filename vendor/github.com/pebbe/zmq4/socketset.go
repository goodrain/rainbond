package zmq4

/*
#include <zmq.h>
#include <stdint.h>
#include <stdlib.h>
#include "zmq4.h"
*/
import "C"

import (
	"time"
	"unsafe"
)

func (soc *Socket) setString(opt C.int, s string) error {
	if !soc.opened {
		return ErrorSocketClosed
	}
	cs := C.CString(s)
	defer C.free(unsafe.Pointer(cs))
	if i, err := C.zmq_setsockopt(soc.soc, opt, unsafe.Pointer(cs), C.size_t(len(s))); i != 0 {
		return errget(err)
	}
	return nil
}

func (soc *Socket) setNullString(opt C.int) error {
	if !soc.opened {
		return ErrorSocketClosed
	}
	if i, err := C.zmq_setsockopt(soc.soc, opt, nil, 0); i != 0 {
		return errget(err)
	}
	return nil
}

func (soc *Socket) setInt(opt C.int, value int) error {
	if !soc.opened {
		return ErrorSocketClosed
	}
	val := C.int(value)
	if i, err := C.zmq_setsockopt(soc.soc, opt, unsafe.Pointer(&val), C.size_t(unsafe.Sizeof(val))); i != 0 {
		return errget(err)
	}
	return nil
}

func (soc *Socket) setInt64(opt C.int, value int64) error {
	if !soc.opened {
		return ErrorSocketClosed
	}
	val := C.int64_t(value)
	if i, err := C.zmq_setsockopt(soc.soc, opt, unsafe.Pointer(&val), C.size_t(unsafe.Sizeof(val))); i != 0 {
		return errget(err)
	}
	return nil
}

func (soc *Socket) setUInt64(opt C.int, value uint64) error {
	if !soc.opened {
		return ErrorSocketClosed
	}
	val := C.uint64_t(value)
	if i, err := C.zmq_setsockopt(soc.soc, opt, unsafe.Pointer(&val), C.size_t(unsafe.Sizeof(val))); i != 0 {
		return errget(err)
	}
	return nil
}

// ZMQ_SNDHWM: Set high water mark for outbound messages
//
// See: http://api.zeromq.org/4-1:zmq-setsockopt#toc39
func (soc *Socket) SetSndhwm(value int) error {
	return soc.setInt(C.ZMQ_SNDHWM, value)
}

// ZMQ_RCVHWM: Set high water mark for inbound messages
//
// See: http://api.zeromq.org/4-1:zmq-setsockopt#toc28
func (soc *Socket) SetRcvhwm(value int) error {
	return soc.setInt(C.ZMQ_RCVHWM, value)
}

// ZMQ_AFFINITY: Set I/O thread affinity
//
// See: http://api.zeromq.org/4-1:zmq-setsockopt#toc3
func (soc *Socket) SetAffinity(value uint64) error {
	return soc.setUInt64(C.ZMQ_AFFINITY, value)
}

// ZMQ_SUBSCRIBE: Establish message filter
//
// See: http://api.zeromq.org/4-1:zmq-setsockopt#toc41
func (soc *Socket) SetSubscribe(filter string) error {
	return soc.setString(C.ZMQ_SUBSCRIBE, filter)
}

// ZMQ_UNSUBSCRIBE: Remove message filter
//
// See: http://api.zeromq.org/4-1:zmq-setsockopt#toc47
func (soc *Socket) SetUnsubscribe(filter string) error {
	return soc.setString(C.ZMQ_UNSUBSCRIBE, filter)
}

// ZMQ_IDENTITY: Set socket identity
//
// See: http://api.zeromq.org/4-1:zmq-setsockopt#toc16
func (soc *Socket) SetIdentity(value string) error {
	return soc.setString(C.ZMQ_IDENTITY, value)
}

// ZMQ_RATE: Set multicast data rate
//
// See: http://api.zeromq.org/4-1:zmq-setsockopt#toc26
func (soc *Socket) SetRate(value int) error {
	return soc.setInt(C.ZMQ_RATE, value)
}

// ZMQ_RECOVERY_IVL: Set multicast recovery interval
//
// See: http://api.zeromq.org/4-1:zmq-setsockopt#toc32
func (soc *Socket) SetRecoveryIvl(value time.Duration) error {
	val := int(value / time.Millisecond)
	return soc.setInt(C.ZMQ_RECOVERY_IVL, val)
}

// ZMQ_SNDBUF: Set kernel transmit buffer size
//
// See: http://api.zeromq.org/4-1:zmq-setsockopt#toc38
func (soc *Socket) SetSndbuf(value int) error {
	return soc.setInt(C.ZMQ_SNDBUF, value)
}

// ZMQ_RCVBUF: Set kernel receive buffer size
//
// See: http://api.zeromq.org/4-1:zmq-setsockopt#toc27
func (soc *Socket) SetRcvbuf(value int) error {
	return soc.setInt(C.ZMQ_RCVBUF, value)
}

// ZMQ_LINGER: Set linger period for socket shutdown
//
// For infinite, use -1
//
// See: http://api.zeromq.org/4-1:zmq-setsockopt#toc19
func (soc *Socket) SetLinger(value time.Duration) error {
	val := int(value / time.Millisecond)
	if value == -1 {
		val = -1
	}
	return soc.setInt(C.ZMQ_LINGER, val)
}

// ZMQ_RECONNECT_IVL: Set reconnection interval
//
// For no reconnection, use -1
//
// See: http://api.zeromq.org/4-1:zmq-setsockopt#toc30
func (soc *Socket) SetReconnectIvl(value time.Duration) error {
	val := int(value / time.Millisecond)
	if value == -1 {
		val = -1
	}
	return soc.setInt(C.ZMQ_RECONNECT_IVL, val)
}

// ZMQ_RECONNECT_IVL_MAX: Set maximum reconnection interval
//
// See: http://api.zeromq.org/4-1:zmq-setsockopt#toc31
func (soc *Socket) SetReconnectIvlMax(value time.Duration) error {
	val := int(value / time.Millisecond)
	return soc.setInt(C.ZMQ_RECONNECT_IVL_MAX, val)
}

// ZMQ_BACKLOG: Set maximum length of the queue of outstanding connections
//
// See: http://api.zeromq.org/4-1:zmq-setsockopt#toc4
func (soc *Socket) SetBacklog(value int) error {
	return soc.setInt(C.ZMQ_BACKLOG, value)
}

// ZMQ_MAXMSGSIZE: Maximum acceptable inbound message size
//
// See: http://api.zeromq.org/4-1:zmq-setsockopt#toc20
func (soc *Socket) SetMaxmsgsize(value int64) error {
	return soc.setInt64(C.ZMQ_MAXMSGSIZE, value)
}

// ZMQ_MULTICAST_HOPS: Maximum network hops for multicast packets
//
// See: http://api.zeromq.org/4-1:zmq-setsockopt#toc21
func (soc *Socket) SetMulticastHops(value int) error {
	return soc.setInt(C.ZMQ_MULTICAST_HOPS, value)
}

// ZMQ_RCVTIMEO: Maximum time before a recv operation returns with EAGAIN
//
// For infinite, use -1
//
// See: http://api.zeromq.org/4-1:zmq-setsockopt#toc29
func (soc *Socket) SetRcvtimeo(value time.Duration) error {
	val := int(value / time.Millisecond)
	if value == -1 {
		val = -1
	}
	return soc.setInt(C.ZMQ_RCVTIMEO, val)
}

// ZMQ_SNDTIMEO: Maximum time before a send operation returns with EAGAIN
//
// For infinite, use -1
//
// See: http://api.zeromq.org/4-1:zmq-setsockopt#toc40
func (soc *Socket) SetSndtimeo(value time.Duration) error {
	val := int(value / time.Millisecond)
	if value == -1 {
		val = -1
	}
	return soc.setInt(C.ZMQ_SNDTIMEO, val)
}

// ZMQ_IPV6: Enable IPv6 on socket
//
// See: http://api.zeromq.org/4-1:zmq-setsockopt#toc18
func (soc *Socket) SetIpv6(value bool) error {
	val := 0
	if value {
		val = 1
	}
	return soc.setInt(C.ZMQ_IPV6, val)
}

// ZMQ_IMMEDIATE: Queue messages only to completed connections
//
// See: http://api.zeromq.org/4-1:zmq-setsockopt#toc17
func (soc *Socket) SetImmediate(value bool) error {
	val := 0
	if value {
		val = 1
	}
	return soc.setInt(C.ZMQ_IMMEDIATE, val)
}

// ZMQ_ROUTER_MANDATORY: accept only routable messages on ROUTER sockets
//
// See: http://api.zeromq.org/4-1:zmq-setsockopt#toc36
func (soc *Socket) SetRouterMandatory(value int) error {
	return soc.setInt(C.ZMQ_ROUTER_MANDATORY, value)
}

// ZMQ_ROUTER_RAW: switch ROUTER socket to raw mode
//
// This option is deprecated since ZeroMQ version 4.1, please use ZMQ_STREAM sockets instead.
//
// See: http://api.zeromq.org/4-1:zmq-setsockopt#toc37
func (soc *Socket) SetRouterRaw(value int) error {
	return soc.setInt(C.ZMQ_ROUTER_RAW, value)
}

// ZMQ_PROBE_ROUTER: bootstrap connections to ROUTER sockets
//
// See: http://api.zeromq.org/4-1:zmq-setsockopt#toc25
func (soc *Socket) SetProbeRouter(value int) error {
	return soc.setInt(C.ZMQ_PROBE_ROUTER, value)
}

// ZMQ_XPUB_VERBOSE: provide all subscription messages on XPUB sockets
//
// See: http://api.zeromq.org/4-1:zmq-setsockopt#toc48
func (soc *Socket) SetXpubVerbose(value int) error {
	return soc.setInt(C.ZMQ_XPUB_VERBOSE, value)
}

// ZMQ_REQ_CORRELATE: match replies with requests
//
// See: http://api.zeromq.org/4-1:zmq-setsockopt#toc33
func (soc *Socket) SetReqCorrelate(value int) error {
	return soc.setInt(C.ZMQ_REQ_CORRELATE, value)
}

// ZMQ_REQ_RELAXED: relax strict alternation between request and reply
//
// See: http://api.zeromq.org/4-1:zmq-setsockopt#toc34
func (soc *Socket) SetReqRelaxed(value int) error {
	return soc.setInt(C.ZMQ_REQ_RELAXED, value)
}

// ZMQ_TCP_KEEPALIVE: Override SO_KEEPALIVE socket option
//
// See: http://api.zeromq.org/4-1:zmq-setsockopt#toc42
func (soc *Socket) SetTcpKeepalive(value int) error {
	return soc.setInt(C.ZMQ_TCP_KEEPALIVE, value)
}

// ZMQ_TCP_KEEPALIVE_IDLE: Override TCP_KEEPCNT(or TCP_KEEPALIVE on some OS)
//
// See: http://api.zeromq.org/4-1:zmq-setsockopt#toc44
func (soc *Socket) SetTcpKeepaliveIdle(value int) error {
	return soc.setInt(C.ZMQ_TCP_KEEPALIVE_IDLE, value)
}

// ZMQ_TCP_KEEPALIVE_CNT: Override TCP_KEEPCNT socket option
//
// See: http://api.zeromq.org/4-1:zmq-setsockopt#toc43
func (soc *Socket) SetTcpKeepaliveCnt(value int) error {
	return soc.setInt(C.ZMQ_TCP_KEEPALIVE_CNT, value)
}

// ZMQ_TCP_KEEPALIVE_INTVL: Override TCP_KEEPINTVL socket option
//
// See: http://api.zeromq.org/4-1:zmq-setsockopt#toc45
func (soc *Socket) SetTcpKeepaliveIntvl(value int) error {
	return soc.setInt(C.ZMQ_TCP_KEEPALIVE_INTVL, value)
}

// ZMQ_TCP_ACCEPT_FILTER: Assign filters to allow new TCP connections
//
// This option is deprecated since ZeroMQ version 4.1, please use authentication via
// the ZAP API and IP address whitelisting / blacklisting.
//
// See: http://api.zeromq.org/4-1:zmq-setsockopt#toc50
func (soc *Socket) SetTcpAcceptFilter(filter string) error {
	if len(filter) == 0 {
		return soc.setNullString(C.ZMQ_TCP_ACCEPT_FILTER)
	}
	return soc.setString(C.ZMQ_TCP_ACCEPT_FILTER, filter)
}

// ZMQ_PLAIN_SERVER: Set PLAIN server role
//
// See: http://api.zeromq.org/4-1:zmq-setsockopt#toc23
func (soc *Socket) SetPlainServer(value int) error {
	return soc.setInt(C.ZMQ_PLAIN_SERVER, value)
}

// ZMQ_PLAIN_USERNAME: Set PLAIN security username
//
// See: http://api.zeromq.org/4-1:zmq-setsockopt#toc24
func (soc *Socket) SetPlainUsername(username string) error {
	if len(username) == 0 {
		return soc.setNullString(C.ZMQ_PLAIN_USERNAME)
	}
	return soc.setString(C.ZMQ_PLAIN_USERNAME, username)
}

// ZMQ_PLAIN_PASSWORD: Set PLAIN security password
//
// See: http://api.zeromq.org/4-1:zmq-setsockopt#toc22
func (soc *Socket) SetPlainPassword(password string) error {
	if len(password) == 0 {
		return soc.setNullString(C.ZMQ_PLAIN_PASSWORD)
	}
	return soc.setString(C.ZMQ_PLAIN_PASSWORD, password)
}

// ZMQ_CURVE_SERVER: Set CURVE server role
//
// See: http://api.zeromq.org/4-1:zmq-setsockopt#toc9
func (soc *Socket) SetCurveServer(value int) error {
	return soc.setInt(C.ZMQ_CURVE_SERVER, value)
}

// ZMQ_CURVE_PUBLICKEY: Set CURVE public key
//
// See: http://api.zeromq.org/4-1:zmq-setsockopt#toc7
func (soc *Socket) SetCurvePublickey(key string) error {
	return soc.setString(C.ZMQ_CURVE_PUBLICKEY, key)
}

// ZMQ_CURVE_SECRETKEY: Set CURVE secret key
//
// See: http://api.zeromq.org/4-1:zmq-setsockopt#toc8
func (soc *Socket) SetCurveSecretkey(key string) error {
	return soc.setString(C.ZMQ_CURVE_SECRETKEY, key)
}

// ZMQ_CURVE_SERVERKEY: Set CURVE server key
//
// See: http://api.zeromq.org/4-1:zmq-setsockopt#toc10
func (soc *Socket) SetCurveServerkey(key string) error {
	return soc.setString(C.ZMQ_CURVE_SERVERKEY, key)
}

// ZMQ_ZAP_DOMAIN: Set RFC 27 authentication domain
//
// See: http://api.zeromq.org/4-1:zmq-setsockopt#toc49
func (soc *Socket) SetZapDomain(domain string) error {
	return soc.setString(C.ZMQ_ZAP_DOMAIN, domain)
}

// ZMQ_CONFLATE: Keep only last message
//
// See: http://api.zeromq.org/4-1:zmq-setsockopt#toc6
func (soc *Socket) SetConflate(value bool) error {
	val := 0
	if value {
		val = 1
	}
	return soc.setInt(C.ZMQ_CONFLATE, val)
}

////////////////////////////////////////////////////////////////
//
// New in ZeroMQ 4.1.0
//
////////////////////////////////////////////////////////////////
//
// + : yes
// D : deprecated
//                                implemented  documented test
// ZMQ_ROUTER_HANDOVER                +            +
// ZMQ_TOS                            +            +
// ZMQ_IPC_FILTER_PID                 D
// ZMQ_IPC_FILTER_UID                 D
// ZMQ_IPC_FILTER_GID                 D
// ZMQ_CONNECT_RID                    +            +
// ZMQ_GSSAPI_SERVER                  +            +
// ZMQ_GSSAPI_PRINCIPAL               +            +
// ZMQ_GSSAPI_SERVICE_PRINCIPAL       +            +
// ZMQ_GSSAPI_PLAINTEXT               +            +
// ZMQ_HANDSHAKE_IVL                  +            +
// ZMQ_SOCKS_PROXY                    +
// ZMQ_XPUB_NODROP                    +
//
////////////////////////////////////////////////////////////////

// ZMQ_ROUTER_HANDOVER: handle duplicate client identities on ROUTER sockets
//
// Returns ErrorNotImplemented41 with ZeroMQ version < 4.1
//
// See: http://api.zeromq.org/4-1:zmq-setsockopt#toc35
func (soc *Socket) SetRouterHandover(value bool) error {
	if minor < 1 {
		return ErrorNotImplemented41
	}
	val := 0
	if value {
		val = 1
	}
	return soc.setInt(C.ZMQ_ROUTER_HANDOVER, val)
}

// ZMQ_TOS: Set the Type-of-Service on socket
//
// Returns ErrorNotImplemented41 with ZeroMQ version < 4.1
//
// See: http://api.zeromq.org/4-1:zmq-setsockopt#toc46
func (soc *Socket) SetTos(value int) error {
	if minor < 1 {
		return ErrorNotImplemented41
	}
	return soc.setInt(C.ZMQ_TOS, value)
}

// ZMQ_CONNECT_RID: Assign the next outbound connection id
//
// Returns ErrorNotImplemented41 with ZeroMQ version < 4.1
//
// See: http://api.zeromq.org/4-1:zmq-setsockopt#toc5
func (soc *Socket) SetConnectRid(value string) error {
	if minor < 1 {
		return ErrorNotImplemented41
	}
	if value == "" {
		return soc.setNullString(C.ZMQ_CONNECT_RID)
	}
	return soc.setString(C.ZMQ_CONNECT_RID, value)
}

// ZMQ_GSSAPI_SERVER: Set GSSAPI server role
//
// Returns ErrorNotImplemented41 with ZeroMQ version < 4.1
//
// See: http://api.zeromq.org/4-1:zmq-setsockopt#toc13
func (soc *Socket) SetGssapiServer(value bool) error {
	if minor < 1 {
		return ErrorNotImplemented41
	}
	val := 0
	if value {
		val = 1
	}
	return soc.setInt(C.ZMQ_GSSAPI_SERVER, val)
}

// ZMQ_GSSAPI_PRINCIPAL: Set name of GSSAPI principal
//
// Returns ErrorNotImplemented41 with ZeroMQ version < 4.1
//
// See: http://api.zeromq.org/4-1:zmq-setsockopt#toc12
func (soc *Socket) SetGssapiPrincipal(value string) error {
	if minor < 1 {
		return ErrorNotImplemented41
	}
	return soc.setString(C.ZMQ_GSSAPI_PRINCIPAL, value)
}

// ZMQ_GSSAPI_SERVICE_PRINCIPAL: Set name of GSSAPI service principal
//
// Returns ErrorNotImplemented41 with ZeroMQ version < 4.1
//
// See: http://api.zeromq.org/4-1:zmq-setsockopt#toc14
func (soc *Socket) SetGssapiServicePrincipal(value string) error {
	if minor < 1 {
		return ErrorNotImplemented41
	}
	return soc.setString(C.ZMQ_GSSAPI_SERVICE_PRINCIPAL, value)
}

// ZMQ_GSSAPI_PLAINTEXT: Disable GSSAPI encryption
//
// Returns ErrorNotImplemented41 with ZeroMQ version < 4.1
//
// See: http://api.zeromq.org/4-1:zmq-setsockopt#toc11
func (soc *Socket) SetGssapiPlaintext(value bool) error {
	if minor < 1 {
		return ErrorNotImplemented41
	}
	val := 0
	if value {
		val = 1
	}
	return soc.setInt(C.ZMQ_GSSAPI_PLAINTEXT, val)
}

// ZMQ_HANDSHAKE_IVL: Set maximum handshake interval
//
// Returns ErrorNotImplemented41 with ZeroMQ version < 4.1
//
// See: http://api.zeromq.org/4-1:zmq-setsockopt#toc15
func (soc *Socket) SetHandshakeIvl(value time.Duration) error {
	if minor < 1 {
		return ErrorNotImplemented41
	}
	val := int(value / time.Millisecond)
	return soc.setInt(C.ZMQ_HANDSHAKE_IVL, val)
}

// ZMQ_SOCKS_PROXY: NOT DOCUMENTED
//
// Returns ErrorNotImplemented41 with ZeroMQ version < 4.1
func (soc *Socket) SetSocksProxy(value string) error {
	if minor < 1 {
		return ErrorNotImplemented41
	}
	if value == "" {
		return soc.setNullString(C.ZMQ_SOCKS_PROXY)
	}
	return soc.setString(C.ZMQ_SOCKS_PROXY, value)
}

// Available since ZeroMQ 4.1, documented since ZeroMQ 4.2

// ZMQ_XPUB_NODROP: do not silently drop messages if SENDHWM is reached
//
// Returns ErrorNotImplemented41 with ZeroMQ version < 4.1
//
// See: http://api.zeromq.org/4-2:zmq-setsockopt#toc60
func (soc *Socket) SetXpubNodrop(value bool) error {
	if minor < 1 {
		return ErrorNotImplemented41
	}
	val := 0
	if value {
		val = 1
	}
	return soc.setInt(C.ZMQ_XPUB_NODROP, val)
}

////////////////////////////////////////////////////////////
//
// New in ZeroMQ 4.2.0
//
////////////////////////////////////////////////////////////////
//
// + : yes
// o : getsockopt only
//                                implemented  documented test
// ZMQ_BLOCKY
// ZMQ_XPUB_MANUAL                      +         +
// ZMQ_XPUB_WELCOME_MSG                 +         +
// ZMQ_STREAM_NOTIFY                    +         +
// ZMQ_INVERT_MATCHING                  +         +
// ZMQ_HEARTBEAT_IVL                    +         +
// ZMQ_HEARTBEAT_TTL                    +         +
// ZMQ_HEARTBEAT_TIMEOUT                +         +
// ZMQ_XPUB_VERBOSER                    +         +
// ZMQ_CONNECT_TIMEOUT                  +         +
// ZMQ_TCP_MAXRT                        +         +
// ZMQ_THREAD_SAFE                      o
// ZMQ_MULTICAST_MAXTPDU                +         +
// ZMQ_VMCI_BUFFER_SIZE                 +         +
// ZMQ_VMCI_BUFFER_MIN_SIZE             +         +
// ZMQ_VMCI_BUFFER_MAX_SIZE             +         +
// ZMQ_VMCI_CONNECT_TIMEOUT             +         +
// ZMQ_USE_FD                           +         +
//
////////////////////////////////////////////////////////////////

// ZMQ_XPUB_MANUAL: change the subscription handling to manual
//
// Returns ErrorNotImplemented42 with ZeroMQ version < 4.2
//
// See: http://api.zeromq.org/4-2:zmq-setsockopt#toc59
func (soc *Socket) SetXpubManual(value int) error {
	if minor < 2 {
		return ErrorNotImplemented42
	}
	return soc.setInt(C.ZMQ_XPUB_MANUAL, value)
}

// ZMQ_XPUB_WELCOME_MSG: set welcome message that will be received by subscriber when connecting
//
// Returns ErrorNotImplemented42 with ZeroMQ version < 4.2
//
// See: http://api.zeromq.org/4-2:zmq-setsockopt#toc61
func (soc *Socket) SetXpubWelcomeMsg(value string) error {
	if minor < 2 {
		return ErrorNotImplemented42
	}
	if value == "" {
		return soc.setNullString(C.ZMQ_XPUB_WELCOME_MSG)
	}
	return soc.setString(C.ZMQ_XPUB_WELCOME_MSG, value)
}

// ZMQ_STREAM_NOTIFY: send connect and disconnect notifications
//
// Returns ErrorNotImplemented42 with ZeroMQ version < 4.2
//
// See: http://api.zeromq.org/4-2:zmq-setsockopt#toc48
func (soc *Socket) SetStreamNotify(value int) error {
	if minor < 2 {
		return ErrorNotImplemented42
	}
	return soc.setInt(C.ZMQ_STREAM_NOTIFY, value)
}

// ZMQ_INVERT_MATCHING: Invert message filtering
//
// Returns ErrorNotImplemented42 with ZeroMQ version < 4.2
//
// See: http://api.zeromq.org/4-2:zmq-setsockopt#toc22
func (soc *Socket) SetInvertMatching(value int) error {
	if minor < 2 {
		return ErrorNotImplemented42
	}
	return soc.setInt(C.ZMQ_INVERT_MATCHING, value)
}

// ZMQ_HEARTBEAT_IVL: Set interval between sending ZMTP heartbeats
//
// Returns ErrorNotImplemented42 with ZeroMQ version < 4.2
//
// See: http://api.zeromq.org/4-2:zmq-setsockopt#toc17
func (soc *Socket) SetHeartbeatIvl(value time.Duration) error {
	if minor < 2 {
		return ErrorNotImplemented42
	}
	val := int(value / time.Millisecond)
	return soc.setInt(C.ZMQ_HEARTBEAT_IVL, val)
}

// ZMQ_HEARTBEAT_TTL: Set the TTL value for ZMTP heartbeats
//
// Returns ErrorNotImplemented42 with ZeroMQ version < 4.2
//
// See: http://api.zeromq.org/4-2:zmq-setsockopt#toc19
func (soc *Socket) SetHeartbeatTtl(value time.Duration) error {
	if minor < 2 {
		return ErrorNotImplemented42
	}
	val := int(value / time.Millisecond)
	return soc.setInt(C.ZMQ_HEARTBEAT_TTL, val)
}

// ZMQ_HEARTBEAT_TIMEOUT: Set timeout for ZMTP heartbeats
//
// Returns ErrorNotImplemented42 with ZeroMQ version < 4.2
//
// See: http://api.zeromq.org/4-2:zmq-setsockopt#toc18
func (soc *Socket) SetHeartbeatTimeout(value time.Duration) error {
	if minor < 2 {
		return ErrorNotImplemented42
	}
	val := int(value / time.Millisecond)
	return soc.setInt(C.ZMQ_HEARTBEAT_TIMEOUT, val)
}

// ZMQ_XPUB_VERBOSER: pass subscribe and unsubscribe messages on XPUB socket
//
// Returns ErrorNotImplemented42 with ZeroMQ version < 4.2
//
// See: http://api.zeromq.org/4-2:zmq-setsockopt#toc58
func (soc *Socket) SetXpubVerboser(value int) error {
	if minor < 2 {
		return ErrorNotImplemented42
	}
	return soc.setInt(C.ZMQ_XPUB_VERBOSER, value)
}

// ZMQ_CONNECT_TIMEOUT: Set connect() timeout
//
// Returns ErrorNotImplemented42 with ZeroMQ version < 4.2
//
// See: http://api.zeromq.org/4-2:zmq-setsockopt#toc7
func (soc *Socket) SetConnectTimeout(value time.Duration) error {
	if minor < 2 {
		return ErrorNotImplemented42
	}
	val := int(value / time.Millisecond)
	return soc.setInt(C.ZMQ_CONNECT_TIMEOUT, val)
}

// ZMQ_TCP_MAXRT: Set TCP Maximum Retransmit Timeout
//
// Returns ErrorNotImplemented42 with ZeroMQ version < 4.2
//
// See: http://api.zeromq.org/4-2:zmq-setsockopt#toc54
func (soc *Socket) SetTcpMaxrt(value time.Duration) error {
	if minor < 2 {
		return ErrorNotImplemented42
	}
	val := int(value / time.Millisecond)
	return soc.setInt(C.ZMQ_TCP_MAXRT, val)
}

// ZMQ_MULTICAST_MAXTPDU: Maximum transport data unit size for multicast packets
//
// Returns ErrorNotImplemented42 with ZeroMQ version < 4.2
//
// See: http://api.zeromq.org/4-2:zmq-setsockopt#toc27
func (soc *Socket) SetMulticastMaxtpdu(value int) error {
	if minor < 2 {
		return ErrorNotImplemented42
	}
	return soc.setInt(C.ZMQ_MULTICAST_MAXTPDU, value)
}

// ZMQ_VMCI_BUFFER_SIZE: Set buffer size of the VMCI socket
//
// Returns ErrorNotImplemented42 with ZeroMQ version < 4.2
//
// See: http://api.zeromq.org/4-2:zmq-setsockopt#toc68
func (soc *Socket) SetVmciBufferSize(value uint64) error {
	if minor < 2 {
		return ErrorNotImplemented42
	}
	return soc.setUInt64(C.ZMQ_VMCI_BUFFER_SIZE, value)
}

// ZMQ_VMCI_BUFFER_MIN_SIZE: Set min buffer size of the VMCI socket
//
// Returns ErrorNotImplemented42 with ZeroMQ version < 4.2
//
// See: http://api.zeromq.org/4-2:zmq-setsockopt#toc69
func (soc *Socket) SetVmciBufferMinSize(value uint64) error {
	if minor < 2 {
		return ErrorNotImplemented42
	}
	return soc.setUInt64(C.ZMQ_VMCI_BUFFER_MIN_SIZE, value)
}

// ZMQ_VMCI_BUFFER_MAX_SIZE: Set max buffer size of the VMCI socket
//
// Returns ErrorNotImplemented42 with ZeroMQ version < 4.2
//
// See: http://api.zeromq.org/4-2:zmq-setsockopt#toc70
func (soc *Socket) SetVmciBufferMaxSize(value uint64) error {
	if minor < 2 {
		return ErrorNotImplemented42
	}
	return soc.setUInt64(C.ZMQ_VMCI_BUFFER_MAX_SIZE, value)
}

// ZMQ_VMCI_CONNECT_TIMEOUT: Set connection timeout of the VMCI socket
//
// Returns ErrorNotImplemented42 with ZeroMQ version < 4.2
//
// See: http://api.zeromq.org/4-2:zmq-setsockopt#toc71
func (soc *Socket) SetVmciConnectTimeout(value time.Duration) error {
	if minor < 2 {
		return ErrorNotImplemented42
	}
	val := int(value / time.Millisecond)
	return soc.setInt(C.ZMQ_VMCI_CONNECT_TIMEOUT, val)
}

// ZMQ_USE_FD: Set the pre-allocated socket file descriptor
//
// Returns ErrorNotImplemented42 with ZeroMQ version < 4.2
//
// See: http://api.zeromq.org/4-2:zmq-setsockopt#toc31
func (soc *Socket) SetUseFd(value int) error {
	if minor < 2 {
		return ErrorNotImplemented42
	}
	return soc.setInt(C.ZMQ_USE_FD, value)
}
