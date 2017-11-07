// +build windows

package zmq4

/*
Sets the scheduling policy for internal context’s thread pool.

This option requires ZeroMQ version 4.1, and is not available on Windows.

Supported values for this option can be found in sched.h file, or at
http://man7.org/linux/man-pages/man2/sched_setscheduler.2.html

This option only applies before creating any sockets on the context.

Default value: -1

Returns ErrorNotImplemented41 with ZeroMQ version < 4.1

Returns ErrorNotImplementedWindows on Windows
*/
func (ctx *Context) SetThreadSchedPolicy(n int) error {
	return ErrorNotImplementedWindows
}

/*
Sets scheduling priority for internal context’s thread pool.

This option requires ZeroMQ version 4.1, and is not available on Windows.

Supported values for this option depend on chosen scheduling policy.
Details can be found in sched.h file, or at
http://man7.org/linux/man-pages/man2/sched_setscheduler.2.html

This option only applies before creating any sockets on the context.

Default value: -1

Returns ErrorNotImplemented41 with ZeroMQ version < 4.1

Returns ErrorNotImplementedWindows on Windows
*/
func (ctx *Context) SetThreadPriority(n int) error {
	return ErrorNotImplementedWindows
}
