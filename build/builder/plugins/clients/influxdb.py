import json
import logging

from _base import BaseHttpClient

logger = logging.getLogger('default')


class InfluxdbAPI(BaseHttpClient):

    def __init__(self, conf, *args, **kwargs):
        BaseHttpClient.__init__(self, *args, **kwargs)
        self.default_headers = {'Connection': 'keep-alive'}
        self.url = 'http://{0}:{1}/db/{2}/series?u={3}&p={4}'.format(conf.host, conf.port, conf.db, conf.user, conf.password)

    def write(self, data):
        if isinstance(data, (list, dict)):
            data = json.dumps(data)
        headers = self.default_headers.copy()
        headers.update({'content-type': 'application/json'})
        try:
            res, body = self._post(self.url, headers, data)
            if 200 <= res.status < 300:
                return True
        except Exception, e:
            logger.exception('client_error', e)
            return False
