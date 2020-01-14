package model

// Defines a group of servers
type Upstream struct {
	Name              string
	UseIpHash         bool // The method ensures that requests from the same client will always be passed to the same server except when this server is unavailable.
	Zone              Zone
	State             string // Specifies a file that keeps the state of the dynamically configurable group.
	Hash              Hash
	Keepalive         int   // Sets the maximum number of idle keepalive connections to upstream servers that are preserved in the cache of each worker process
	KeepaliveRequests int   // Default 100. Sets the maximum number of requests that can be served through one keepalive connection.
	KeepaliveTimeout  Time  // Default 60s. Sets a timeout during which an idle keepalive connection to an upstream server will stay open.
	UseNtlm           bool  // Allows proxying requests with NTLM Authentication.
	UseLeastConn      bool  // Pass the request to the sever with the least numbers of connections.
	Queue             Queue // TODO [emerg] unknown directive "queue"
	Random            Random
	Servers           []UServer
	// TODO least_time        LeastTime
	// TODO sticky
	// TODO sticky_cookie_insert
}

//Validation validation nginx parameters
func (u Upstream) Validation() error {
	return nil
}

// Defines the name and size of the shared memory zone
// that keeps the group’s configuration and run-time state that are shared between worker processes
type Zone struct {
	Name string
	Size Size
}

// Specifies a load balancing method for a server group
// where the client-server mapping is based on the hashed key value
type Hash struct {
	Key           bool // The key can contain text, variables, and their combinations.
	UseConsistent bool // If the consistent parameter is specified the ketama consistent hashing method will be used instead.
}

// If an upstream server cannot be selected immediately while processing a request,
// the request will be placed into the queue
type Queue struct {
	Num     int  // The maximum number of requests
	Timeout Time // Default 60s. The time a request can be kept in the queue.
}

type Random struct {
	UseRandom bool
	UseTwo    bool   // The optional two parameter instructs nginx to randomly select two servers and then choose a server using the specified method.
	Method    string // The default method is least_conn.
}

// Defines the address and other parameters of a server in upstream
type UServer struct {
	Address string
	Params  Params
}

// parameters of a server in upstream
type Params struct {
	Weight      int    // Default 1. Sets the weight of the server.
	MaxConns    int    // Default value is zero, meaning there is no limit. Limits the maximum number of simultaneous active connections to the proxied server.
	MaxFails    int    // Sets the number of unsuccessful attempts to communicate with the server.
	FailTimeout string // default 10s. The period of time the server will be considered unavailable.
	UseBackup   bool   // Marks the server as a backup server.
	UseDown     bool   // Marks the server as permanently unavailable.
	UseResolve  bool   // Monitors changes of the IP addresses that correspond to a domain name of the server, and automatically modifies the upstream configuration without the need of restarting nginx.
	Route       string // Sets the server route name.
	Service     string // Enables resolving of DNS SRV records and sets the service name
	SlowStart   Time   // Sets the time during which the server will recover its weight from zero to a nominal value, when unhealthy server becomes healthy, or when the server becomes available after a period of time it was considered unavailable.
	UseDrain    bool   // Puts the server into the “draining” mode
}
