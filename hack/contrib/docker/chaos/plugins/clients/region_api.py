import logging
import os
import json
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
        body["status"]="success"
        logger.info("publish app to ys?{}".format(body["dest_ys"]))
        res, body = self._post(url, self.default_headers, json.dumps(body))
        return res, body
    def service_publish_failure_region(self, body):
        # url = self.base_url + '/api/tenants/services/publish'
        url = self.base_url+ '/publish'
        body["status"]="failure"
        logger.info("publish app to ys?{}".format(body["dest_ys"]))
        res, body = self._post(url, self.default_headers, json.dumps(body))
        return res, body
    def service_publish_new_region(self, body):
        # url = self.base_url + '/api/tenants/services/publish'
        url = self.base_url+ '/publish'
        body["status"]="pushing"
        logger.info("publish app to ys?{}".format(body["dest_ys"]))
        res, body = self._post(url, self.default_headers, json.dumps(body))
        return res, body
    def code_check_region(self, body):

        # url = self.base_url + '/api/tenants/services/codecheck'
        url = self.base_url+'/codecheck'
        print body
        res, body = self._post(url, self.default_headers, body)
        return res, body


    def update_service_region(self, service_id, body):
        url = self.base_url+'/codecheck/service/{0}'.format(service_id)
        res, body = self._put(url, self.default_headers, body)

    def update_version_region(self, body):
        url = self.base_url+'/version'
        res, body = self._post(url, self.default_headers, body)


    def update_version_event(self, event_id,body):
        url = self.base_url+'/version/event/{0}'.format(event_id)
        res, body = self._post(url, self.default_headers, body)