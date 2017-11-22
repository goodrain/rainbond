from addict import Dict
from _base import SuperHttpClient

import logging
logger = logging.getLogger('default')


class RegistryAPI(SuperHttpClient):

    def __init__(self, conf=None, host=None, *arg, **kwargs):
        if conf is not None:
            host = conf.host

        super(RegistryAPI, self).__init__(host)
        self.apitype = 'registry'
        self.default_headers = {"Content-Type": "application/json", "Connection": "close"}

        self.log_topic = None

    def set_log_topic(self, topic):
        self.log_topic = topic

    def delete_image(self, image):
        data = self.parse_image(image)
        digest = self.get_manifest_digest(data)

        url = self.base_url + '/v2/{0}/manifests/{1}'.format(data.name, digest)
        res, body = self._delete(url, headers=self.default_headers)

    def get_manifest_digest(self, data):
        url = self.base_url + '/v2/{0}/manifests/{1}'.format(data.name, data.tag)
        res, body = self._get(url, headers=self.default_headers)
        return res['docker-content-digest']

    def rename_image(self, image, tag=None):
        data = self.parse_image(image)
        end_name = '{0}/{1}'.format(self.host, data.name)
        if tag is not None:
            end_name = '{0}:{1}'.format(end_name, tag)
        elif data.tag is not None:
            end_name = '{0}:{1}'.format(end_name, data.tag)
        return end_name

    def exist_image(self, image):
        data = self.parse_image(image)
        url = self.base_url + '/v2/{0}/manifests/{1}'.format(data.name, data.tag)
        try:
            res, body = self._get(url, headers=self.default_headers)
            is_exist = True
        except self.CallApiError, e:
            if e.status == 404:
                is_exist = False
            else:
                raise e

        if self.log_topic is not None:
            logger.info(self.log_topic, "check image {0} is or not exists on {1}, result: {2}".format(image, self.host, is_exist))

        return is_exist

    def parse_image(self, image):
        if '/' in image:
            host, full_name = image.split('/', 1)
        else:
            host, full_name = (None, image)

        if ':' in full_name:
            name, tag = full_name.split(':', 1)
        else:
            name, tag = (full_name, 'latest')

        return Dict(host=host, name=name, tag=tag)
