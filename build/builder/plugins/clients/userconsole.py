import logging
import os
from _base import BaseHttpClient

logger = logging.getLogger('default')


class UserConsoleAPI(BaseHttpClient):
    def __init__(self, conf=None, *args, **kwargs):
        super(UserConsoleAPI, self).__init__()
        self._name = 'region'
        self.default_headers = {"Content-Type": "application/json"}
        if conf["token"] is not None:
            self.default_headers.update({"Authorization": "Token {}".format(conf["token"])})
        if conf is None:
            self.base_url = "https://user.goodrain.com"
        else:
            self.base_url = conf["url"]
    
    def update_service(self, service_id, body):
        #todo 127.0.0.1:3333/api/codecheck

        # url = self.base_url + '/api/services/{0}'.format(service_id)
        url = 'http://127.0.0.1:3228/api/codecheck/{0}'.format(service_id)
        res, body = self._put(url, self.default_headers, body)
    
    def update_service_prop(self, service_id, body):
        url = self.base_url + '/api/services/{0}'.format(service_id)
        res, body = self._post(url, self.default_headers, body)
        return res, body
    
    def codecheck(self, service_id, body):
        pass
    
    def get_tenants(self, body, headers=None):
        url = self.base_url + '/api/tenants/all-members'
        if headers is None:
            res, body = self._post(url, self.default_headers, body)
        else:
            res, body = self._post(url, headers, body)
        return res, body
    
    def get_tenant(self, body):
        url = self.base_url + '/api/tenants/member'
        res, body = self._post(url, self.default_headers, body)
        return res, body
    
    def hibernate_service(self, body):
        url = self.base_url + '/api/tenants/services/hibernate'
        res, body = self._put(url, self.default_headers, body)
        return res, body
    
    def stat_service(self, body):
        url = self.base_url + '/api/tenants/services/statics'
        res, body = self._post(url, self.default_headers, body)
        return res, body
    
    def code_check(self, body):
        #todo 127.0.0.1:3333/api/codecheck
        # url = self.base_url + '/api/tenants/services/codecheck'
        url = 'http://127.0.0.1:3228/api/codecheck'
        res, body = self._post(url, self.default_headers, body)
        return res, body
    
    def service_publish_success(self, body):
        url = self.base_url + '/api/tenants/services/publish'
        res, body = self._post(url, self.default_headers, body)
        return res, body
    
    def get_region_rules(self):
        region = os.environ.get('REGION_TAG')
        url = self.base_url + '/api/rules/' + region
        res, body = self._get(url, self.default_headers)
        return res, body
    
    def post_rule_instance(self, rule_id, body):
        rule_id_str = str(rule_id)
        url = self.base_url + '/api/rules/' + rule_id_str + '/instance'
        res, body = self._post(url, self.default_headers, body)
        return res, body
    
    def post_rule_history(self, rule_id, body):
        rule_id_str = str(rule_id)
        url = self.base_url + '/api/rules/' + rule_id_str + '/history'
        res, body = self._put(url, self.default_headers, body)
        return res, body
    
    def get_service_info(self, service_id):
        url = self.base_url + '/api/services/{0}/info'.format(service_id)
        res, body = self._get(url, self.default_headers)
        return res, body
