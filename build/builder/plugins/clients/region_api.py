import logging
import os
from _base import BaseHttpClient

logger = logging.getLogger('default')


class RegionBackAPI(BaseHttpClient):
    def __init__(self, conf=None, *args, **kwargs):
        super(RegionBackAPI, self).__init__()
        self._name = 'region'
        self.default_headers = {"Content-Type": "application/json"}
        if conf["token"] is not None:
            self.default_headers.update({
                "Authorization":
                    "Token {}".format(conf["token"])
            })
        if conf is None:
            self.base_url = "http://localhost:3228/v2/builder"
        else:
            self.base_url = conf["url"]

    def update_service(self, service_id, body):
        #todo 127.0.0.1:3333/api/codecheck

        url = self.base_url + '/api/services/{0}'.format(service_id)
        # url = 'http://127.0.0.1:3228/api/codecheck/{0}'.format(service_id)
        res, body = self._put(url, self.default_headers, body)


    def code_check_region(self, body):

        # url = self.base_url + '/api/tenants/services/codecheck'
        url = 'http://127.0.0.1:3228/v2/builder/codecheck'
        res, body = self._post(url, self.default_headers, body)
        return res, body
