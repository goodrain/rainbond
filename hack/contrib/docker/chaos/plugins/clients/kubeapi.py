from _base import BaseHttpClient


class KubernetesApi(BaseHttpClient):

    def __init__(self, conf=None, *arg, **kwargs):
        super(KubernetesApi, self).__init__()
        self._name = 'kubeapi'
        self.base_url = conf.url
        self.default_headers = {"Content-Type": "application/json"}

    def get_rc(self, tenant_id, replica_id):
        url = self.base_url + "/namespaces/{0}/replicationcontrollers/{1}".format(tenant_id, replica_id)
        res, body = self._get(url, self.default_headers)
        return body
