# -*- coding: utf8 -*-


def get_logging(conf):
    DEFAULT_HANDLERS = conf.get('DEFAULT_HANDLERS', ["console"])

    ZMQ_LOG_ADDRESS = conf["zmq"]['service_pub']["address"]

    EVENT_LOG_ADDRESS = conf.get("EVENT_LOG_ADDRESS", "tcp://127.0.0.1:6366")

    LOGGING = {
        'version': 1,
        'disable_existing_loggers': False,
        'filters': {},
        'formatters': {
            'standard': {
                'format': "%(asctime)s [%(levelname)s] localhost [%(funcName)s] %(pathname)s:%(lineno)s %(message)s",
                'datefmt': "%Y-%m-%d %H:%M:%S"
            },
            'zmq_formatter': {
                'format': "%(asctime)s [%(levelname)s] %(hostname)s [%(funcName)s] %(pathname)s:%(lineno)s %(message)s",
                'datefmt': "%Y-%m-%d %H:%M:%S"
            },
        },
        'handlers': {
            'console': {
                'level': 'DEBUG',
                'class': 'logging.StreamHandler',
                'formatter': 'standard',
            },
            'zmq_handler': {
                'level': 'DEBUG',
                'class': 'utils.log.ZmqHandler',
                'address': ZMQ_LOG_ADDRESS,
                'root_topic': 'labor',
                'formatter': 'zmq_formatter',
            },
            'event_handler': {
                'level': 'DEBUG',
                'class': 'utils.log.EventHandler',
                'address': EVENT_LOG_ADDRESS,
            },
        },
        'loggers': {
            'main': {
                'handlers': ['console'],
                'level': 'DEBUG',
                'propagate': True,
            },
            'default': {
                'handlers': DEFAULT_HANDLERS,
                'level': 'DEBUG',
                'propagate': False,
            },
            'event': {
                'handlers': ['event_handler'],
                'level': 'DEBUG',
                'propagate': False,
            },
        },
    }
    return LOGGING
