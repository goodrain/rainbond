# -*- coding: utf8 -*-
import logging
import json
import os
from _base import BaseHttpClient

logger = logging.getLogger('default')


class RegionAPI(BaseHttpClient):

    def __init__(self, conf=None, *arg, **kwargs):
        super(RegionAPI, self).__init__()
        self._name = 'region'
        self.default_headers = {"Content-Type": "application/json"}
        if conf["token"] is not None:
            self.default_headers.update({"Authorization": "Token {}".format(conf["token"])})
        if conf["url"] is None:
            self.base_url = 'http://region.goodrain.me:8888'
        else:
            self.base_url = conf["url"]

    def upgrade_service(self, service_id, body):
        url = self.base_url + \
            '/v1/services/lifecycle/{0}/upgrade/'.format(service_id)
        res, body = self._post(url, self.default_headers, body)
        return res, body

    def rolling_upgrade_service(self, service_id, body):
        url = self.base_url + \
            '/v1/services/lifecycle/{0}/upgrade/'.format(service_id)
        res, body = self._put(url, self.default_headers, body)
        return res, body

    def deploy_service(self, service_id, body):
        url = self.base_url + \
            '/v1/services/lifecycle/{0}/deploy/'.format(service_id)
        res, body = self._post(url, self.default_headers, body)
        return res, body

    def start_service(self, service_id, body):
        url = self.base_url + \
            '/v1/services/lifecycle/{0}/start/'.format(service_id)
        res, body = self._post(url, self.default_headers, body)
        return res, body

    def system_pause(self, tenant_id):
        url = self.base_url + '/v1/tenants/{0}/system-pause'.format(tenant_id)
        res, body = self._post(url, self.default_headers)
        return res, body

    def stop_service(self, service_id):
        url = self.base_url + \
            '/v1/services/lifecycle/{0}/stop/'.format(service_id)
        tmp_body = json.dumps({
            "event_id": "system"
        })
        res, body = self._post(url, self.default_headers, body=tmp_body)
        return res, body

    def update_b_event(self, service_id, body):
        url = self.base_url + \
            '/v1/services/lifecycle/{0}/beanstalk/'.format(service_id)
        res, body = self._post(url, self.default_headers, body)
        return body

    def update_event(self, body):
        url = self.base_url + '/v1/events'
        res, body = self._put(url, self.default_headers, body)
        return body

    def get_history_pods(self, service_id):
        url = self.base_url + '/v1/services/lifecycle/{0}/history_pods'.format(service_id)
        res, body = self._get(url, self.default_headers)
        return body

    def clean_history_pods(self, service_id):
        url = self.base_url + '/v1/services/lifecycle/{0}/history_pods'.format(service_id)
        res, body = self._delete(url, self.default_headers)
        return body

    def get_lb_ngx_info(self, tenant_name, service_name):
        url = self.base_url + '/v1/lb/ngx-info/{0}/{1}'.format(tenant_name, service_name)
        res, body = self._get(url, self.default_headers)
        return body

    def renew_lb_ngx_info(self, body):
        url = self.base_url + '/v1/lb/ngx-info'
        res, body = self._post(url, self.default_headers, body)
        return body
    
    def set_service_running(self, service_id):
        url = self.base_url + \
            '/v1/services/lifecycle/{0}/set-running/'.format(service_id)
        res, body = self._post(url, self.default_headers)
        return res, body
    
    def is_service_running(self, service_id):
        url = self.base_url + \
            '/v1/services/lifecycle/{0}/status/'.format(service_id)
        res, body = self._post(url, self.default_headers)
        return res, body
    
    def opentsdbQuery(self, start, queries):
        try:
            url = self.base_url + "/v1/statistic/opentsdb/query"
            data = {"start": start, "queries": queries}
            res, body = self._post(url, self.default_headers, json.dumps(data))
            dps = body[0]['dps']
            return dps
        except IndexError:
            logger.info('tsdb_query', "request: {0}".format(url))
            logger.info('tsdb_query', "response: {0} ====== {1}".format(res, body))
            return None
