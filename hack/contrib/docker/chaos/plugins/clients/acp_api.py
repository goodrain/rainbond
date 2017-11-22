# -*- coding: utf8 -*-
import logging
import json
import os
from _base import BaseHttpClient

logger = logging.getLogger('default')


class ACPAPI(BaseHttpClient):
    def __init__(self, conf=None, *arg, **kwargs):
        super(ACPAPI, self).__init__()
        self._name = 'region'
        self.default_headers = {"Content-Type": "application/json"}
        if conf["token"] is not None:
            self.default_headers.update({
                "Authorization":
                "Token {}".format(conf["token"])
            })
        if conf["url"] is None:
            self.base_url = 'http://region.goodrain.me:8888'
        else:
            self.base_url = conf["url"]

    def upgrade_service(self, tenant_name, service_alias, body):
        url = self.base_url + \
            '/v2/tenants/{0}/services/{1}/upgrade'.format(tenant_name, service_alias)
        logger.exception("url is {}".format(url))
        res, body = self._post(url, self.default_headers, body)
        return res, body

    def start_service(self, tenant_name, service_alias, event_id):
        url = self.base_url + \
            '/v2/tenants/{0}/services/{1}/start'.format(tenant_name, service_alias)
        res, body = self._post(url, self.default_headers, json.dumps({"event_id": event_id}))
        return res, body

    def update_iamge(self, tenant_name, service_alias, image_name):
        url = self.base_url + \
            '/v2/tenants/{0}/services/{1}'.format(tenant_name, service_alias)
        res, body = self._put(url, self.default_headers, json.dumps({"image_name": image_name}))
        return res, body
