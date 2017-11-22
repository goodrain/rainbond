# -*- coding: utf8 -*-
import socket
import logging
import zmq
import sys
from zmq.log.handlers import PUBHandler
from zmq.utils.strtypes import cast_bytes
import structlog

TOPIC_DELIM = " :: "
HOSTNAME = socket.gethostname()


def do_stdout(arg, end=True):
    sys.stdout.write('{}'.format(arg))
    if end:
        sys.stdout.write("\n")
    sys.stdout.flush()


class MyLogRecord(logging.LogRecord):
    def __init__(self,
                 name,
                 level,
                 pathname,
                 lineno,
                 msg,
                 args,
                 exc_info,
                 func=None):
        if args and '%' not in msg:
            arg = args[0]

            try:
                if isinstance(arg, unicode):
                    msg = u'{0}{1}{2}'.format(msg, TOPIC_DELIM, arg)
                else:
                    msg = '{0}{1}{2}'.format(msg, TOPIC_DELIM, arg)
            except Exception:
                print "type is %s" % type(arg)
                print "arg is", arg
            finally:
                args = []

        super(MyLogRecord, self).__init__(
            name, level, pathname, lineno, msg, args, exc_info, func=func)


class ZmqHandler(PUBHandler):
    def __init__(self, address, root_topic):
        logging.Handler.__init__(self)
        logging.LogRecord = MyLogRecord
        self.ctx = zmq.Context()
        self.socket = self.ctx.socket(zmq.PUB)
        self.socket.connect(address)
        if '.' in root_topic:
            raise AttributeError(
                "root_topic should not contains any '.', provided '%s'" %
                root_topic)
        self.root_topic = root_topic

    def format(self, record):
        fmt = self.formatter
        return fmt.format(record)

    def emit(self, record):
        """Emit a log message on my socket."""

        try:
            topic, record.msg = record.msg.split(TOPIC_DELIM, 1)
        except Exception:
            topic = "untopic"

        record.__dict__['hostname'] = HOSTNAME

        try:
            bmsg = cast_bytes(self.format(record))
        except Exception:
            self.handleError(record)
            return

        topic_list = [self.root_topic, topic]

        btopic = b'.'.join(cast_bytes(t) for t in topic_list)
        blevel = cast_bytes(record.levelname)

        self.socket.send_multipart([btopic, blevel, bmsg])


class EventHandler(PUBHandler):
    def __init__(self, address):
        logging.Handler.__init__(self)
        logging.LogRecord = logging.LogRecord
        self.ctx = zmq.Context()
        self.socket = self.ctx.socket(zmq.REQ)
        self.socket.connect(address)
        self.address = address

    def emit(self, record):
        """Emit a log message on my socket."""
        try:
            bmsg = cast_bytes(record.msg)
        except Exception:
            self.handleError(record)
            return
        retry = 2
        while retry > 0:
            try:
                self.socket.send(bmsg)
                poller = zmq.Poller()
                poller.register(self.socket, flags=zmq.POLLIN)
                # 0.5s超时
                polled = poller.poll(500)
                if len(polled) > 0:
                    rep = self.socket.recv()
                    if rep != "OK":
                        retry -= 1
                        continue
                    else:
                        return
                else:
                    retry -= 1
                    continue
            except Exception as e:
                do_stdout("log send error {}".format(e))
                self.handleError(record)
                self.socket.close()
                self.socket = self.ctx.socket(zmq.REQ)
                self.socket.connect(self.address)
                return


class EventLog:
    def __init__(self):
        WrappedDictClass = structlog.threadlocal.wrap_dict(dict)
        structlog.configure(
            processors=[
                self.add_log_level,
                structlog.processors.TimeStamper(
                    fmt="iso", utc=False, key="time"), self.event2message,
                structlog.processors.JSONRenderer()
            ],
            context_class=WrappedDictClass(),
            logger_factory=structlog.stdlib.LoggerFactory(),
            wrapper_class=structlog.stdlib.BoundLogger,
            cache_logger_on_first_use=True, )
        self.log = structlog.get_logger("event")

    def get_logger(self):
        return self.log

    def bind(self, **kwargs):
        return self.log.bind(**kwargs)

    def add_log_level(self, logger, method_name, event_dict):
        """
        Add the log level to the event dict.
        """
        if method_name == 'warn':
            # The stdlib has an alias
            method_name = 'warning'

        event_dict['level'] = method_name
        return event_dict

    def event2message(self, logger, method_name, event_dict):
        """
        event->message
        :param _:
        :param __:
        :param event_dict:
        :return:
        """
        if 'event' in event_dict:
            event_dict['message'] = event_dict['event']
            del event_dict['event']
        return event_dict
