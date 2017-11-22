import json
import logging

from _base import BaseHttpClient

logger = logging.getLogger('default')


class OpentsdbAPI(BaseHttpClient):

    def __init__(self, conf, *args, **kwargs):
        BaseHttpClient.__init__(self, *args, **kwargs)
        self.default_headers = {'Connection': 'keep-alive', 'content-type': 'application/json'}
        self.url = 'http://{0}:{1}/api/put'.format(conf.host, conf.port)

    def write(self, data):
        if isinstance(data, (list, dict)):
            data = json.dumps(data)

        try:
            res, body = self._post(self.url, self.default_headers, data)
            return True
        except self.CallApiError, e:
            logger.exception('client_error', e)
            return False
