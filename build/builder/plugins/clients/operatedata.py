import json
import logging

from _base import BaseHttpClient

logger = logging.getLogger('default')


class OperateDataApi(BaseHttpClient):

    def __init__(self, conf=None, *args, **kwargs):
        super(OperateDataApi, self).__init__()
        self.default_headers = {"Content-Type": "application/json"}
        if conf is None:
            self.base_url = "http://op_console.goodrain.ali-sh.goodrain.net:10080"
        else:
            self.base_url = conf.url
    
    def send_log(self, body):
        url = self.base_url + '/api/operate-log/'
        res, body = self._post(url, self.default_headers, body)
        return res, body
    
    def send_container(self, body):
        url = self.base_url + '/api/operate-container/'
        res, body = self._post(url, self.default_headers, body)
        return res, body
    
    def send_evnetdata(self, body):
        url = self.base_url + '/api/operate-event/'
        res, body = self._post(url, self.default_headers, body)
        return res, body
    
    def send_container_memory(self, body):
        url = self.base_url + '/api/operate-container-memory/'
        res, body = self._post(url, self.default_headers, body)
        return res, body
    
    def send_service_running(self, body):
        url = self.base_url + '/api/operate-running-statics/'
        res, body = self._post(url, self.default_headers, body)
        return res, body
    
