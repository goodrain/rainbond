/*
A Go interface to ZeroMQ (zmq, 0mq) version 4.

This includes partial support for ZeroMQ 4.2 DRAFT. The API pertaining
to this support is subject to change.

For ZeroMQ version 3, see: http://github.com/pebbe/zmq3

For ZeroMQ version 2, see: http://github.com/pebbe/zmq2

http://www.zeromq.org/

See also the wiki: https://github.com/pebbe/zmq4/wiki

A note on the use of a context:

This package provides a default context. This is what will be used by
the functions without a context receiver, that create a socket or
manipulate the context. Package developers that import this package
should probably not use the default context with its associated
functions, but create their own context(s). See: type Context.
*/
package zmq4
