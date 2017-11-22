import time
from functools import wraps

import logging

logger = logging.getLogger('default')


def method_perf_time(func):

    @wraps(func)
    def wrapper(self, *args, **kwargs):
        class_name = self.__class__.__name__
        start_time = time.time()
        ret = func(self, *args, **kwargs)
        end_time = time.time()
        use_time = end_time - start_time
        logger.debug('perf', "class {0}, function {1}, cost_time {2}".format(class_name, func.__name__, use_time))
        return ret

    return wrapper


def mirror_exec(func):

    @wraps(func)
    def wrapper(self, *args, **kwargs):
        method_name = func.__name__
        self.update_location(method_name)
        method = getattr(self.conn, method_name)
        return func(self, execute=method, *args, **kwargs)
    return wrapper
