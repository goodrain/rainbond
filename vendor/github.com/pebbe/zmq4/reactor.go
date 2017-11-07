package zmq4

import (
	"errors"
	"fmt"
	"time"
)

type reactor_socket struct {
	e State
	f func(State) error
}

type reactor_channel struct {
	ch    <-chan interface{}
	f     func(interface{}) error
	limit int
}

type Reactor struct {
	sockets  map[*Socket]*reactor_socket
	channels map[uint64]*reactor_channel
	p        *Poller
	idx      uint64
	remove   []uint64
	verbose  bool
}

/*
Create a reactor to mix the handling of sockets and channels (timers or other channels).

Example:

    reactor := zmq.NewReactor()
    reactor.AddSocket(socket1, zmq.POLLIN, socket1_handler)
    reactor.AddSocket(socket2, zmq.POLLIN, socket2_handler)
    reactor.AddChannelTime(time.Tick(time.Second), 1, ticker_handler)
    reactor.Run(time.Second)
*/
func NewReactor() *Reactor {
	r := &Reactor{
		sockets:  make(map[*Socket]*reactor_socket),
		channels: make(map[uint64]*reactor_channel),
		p:        NewPoller(),
		remove:   make([]uint64, 0),
	}
	return r
}

// Add socket handler to the reactor.
//
// You can have only one handler per socket. Adding a second one will remove the first.
//
// The handler receives the socket state as an argument: POLLIN, POLLOUT, or both.
func (r *Reactor) AddSocket(soc *Socket, events State, handler func(State) error) {
	r.RemoveSocket(soc)
	r.sockets[soc] = &reactor_socket{e: events, f: handler}
	r.p.Add(soc, events)
}

// Remove a socket handler from the reactor.
func (r *Reactor) RemoveSocket(soc *Socket) {
	if _, ok := r.sockets[soc]; ok {
		delete(r.sockets, soc)
		// rebuild poller
		r.p = NewPoller()
		for s, props := range r.sockets {
			r.p.Add(s, props.e)
		}
	}
}

// Add channel handler to the reactor.
//
// Returns id of added handler, that can be used later to remove it.
//
// If limit is positive, at most this many items will be handled in each run through the main loop,
// otherwise it will process as many items as possible.
//
// The handler function receives the value received from the channel.
func (r *Reactor) AddChannel(ch <-chan interface{}, limit int, handler func(interface{}) error) (id uint64) {
	r.idx++
	id = r.idx
	r.channels[id] = &reactor_channel{ch: ch, f: handler, limit: limit}
	return
}

// This function wraps AddChannel, using a channel of type time.Time instead of type interface{}.
func (r *Reactor) AddChannelTime(ch <-chan time.Time, limit int, handler func(interface{}) error) (id uint64) {
	ch2 := make(chan interface{})
	go func() {
		for {
			a, ok := <-ch
			if !ok {
				close(ch2)
				break
			}
			ch2 <- a
		}
	}()
	return r.AddChannel(ch2, limit, handler)
}

// Remove a channel from the reactor.
//
// Closed channels are removed automatically.
func (r *Reactor) RemoveChannel(id uint64) {
	r.remove = append(r.remove, id)
}

func (r *Reactor) SetVerbose(verbose bool) {
	r.verbose = verbose
}

// Run the reactor.
//
// The interval determines the time-out on the polling of sockets.
// Interval must be positive if there are channels.
// If there are no channels, you can set interval to -1.
//
// The run alternates between polling/handling sockets (using the interval as timeout),
// and reading/handling channels. The reading of channels is without time-out: if there
// is no activity on any channel, the run continues to poll sockets immediately.
//
// The run exits when any handler returns an error, returning that same error.
func (r *Reactor) Run(interval time.Duration) (err error) {
	for {

		// process requests to remove channels
		for _, id := range r.remove {
			delete(r.channels, id)
		}
		r.remove = r.remove[0:0]

	CHANNELS:
		for id, ch := range r.channels {
			limit := ch.limit
			for {
				select {
				case val, ok := <-ch.ch:
					if !ok {
						if r.verbose {
							fmt.Printf("Reactor(%p) removing closed channel %d\n", r, id)
						}
						r.RemoveChannel(id)
						continue CHANNELS
					}
					if r.verbose {
						fmt.Printf("Reactor(%p) channel %d: %q\n", r, id, val)
					}
					err = ch.f(val)
					if err != nil {
						return
					}
					if ch.limit > 0 {
						limit--
						if limit == 0 {
							continue CHANNELS
						}
					}
				default:
					continue CHANNELS
				}
			}
		}

		if len(r.channels) > 0 && interval < 0 {
			return errors.New("There are channels, but polling time-out is infinite")
		}

		if len(r.sockets) == 0 {
			if len(r.channels) == 0 {
				return errors.New("No sockets to poll, no channels to read")
			}
			time.Sleep(interval)
			continue
		}

		polled, e := r.p.Poll(interval)
		if e != nil {
			return e
		}
		for _, item := range polled {
			if r.verbose {
				fmt.Printf("Reactor(%p) %v\n", r, item)
			}
			err = r.sockets[item.Socket].f(item.Events)
			if err != nil {
				return
			}
		}
	}
	return
}
