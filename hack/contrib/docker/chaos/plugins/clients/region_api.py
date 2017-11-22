import logging
import os
from _base import BaseHttpClient

logger = logging.getLogger('default')


class RegionBackAPI(BaseHttpClient):
    def __init__(self, conf=None, *args, **kwargs):
        super(RegionBackAPI, self).__init__()
        self._name = 'region'
        self.default_headers = {"Content-Type": "application/json"}
        if conf is None:
            self.base_url = "http://localhost:3228/v2/builder"
        else:
            self.base_url = conf["url"]

    def service_publish_success_region(self, body):
        # url = self.base_url + '/api/tenants/services/publish'
        url = self.base_url+ '/publish'
        res, body = self._post(url, self.default_headers, body)
        return res, body

    def code_check_region(self, body):

        # url = self.base_url + '/api/tenants/services/codecheck'
        url = self.base_url+'/codecheck'
        print body
        res, body = self._post(url, self.default_headers, body)
        return res, body
