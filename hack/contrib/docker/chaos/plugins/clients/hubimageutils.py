#!/usr/bin/env python
# -*- coding: utf8 -*-

import requests
import json
from addict import Dict
import logging
logger = logging.getLogger('default')


class HubUtils:
    """ cloud market image hub upload/download interface """
    def __init__(self, image_config):
        self.username = image_config.get("oss_username")
        self.password = image_config.get("oss_password")
        self.namespace = image_config.get('oss_namespace')
        self.host = image_config.get('oss_host')
        self.cert = image_config.get('oss_cart')
        self.dockersearch = '/api/v0/index/dockersearch'  # get
        self.search = '/api/v0/index/search'  # get
        self.reindex = '/api/v0/index/reindex'  # POST

        self.repo_error = {
            400: '名称已经存在',
            401: '客户端未授权',
            403: '客户端无权限',
            404: '账户不存在',
            409: '未配置管理账户',
        }

    def rename_image(self, image, tag=None, namespace=None):
        data = self.parse_image(image)
        if not namespace:
            namespace = self.namespace
        # goodrain.me/xxx:tagx hub.goodrain.com/goodrain/xxx:tagx
        end_name = '{0}/{1}'.format(self.host + '/' + namespace, data.name)
        if tag is not None:
            end_name = '{0}:{1}'.format(end_name, tag)
        elif data.tag is not None:
            end_name = '{0}:{1}'.format(end_name, data.tag)
        return end_name

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

    def check(self, image_name, tag_name, namespace=None):
        # 1, 检查命名空间是否存在,
        # 2, 存在,检查tag_name是否存在
        # 3, 不存在,新建空间
        if not namespace:
            namespace = self.namespace
        repositories_url = '/api/v0/repositories/%s' % namespace
        url = 'https://' + self.host + '/' + repositories_url + '/' + image_name
        headers = {'content-type': 'application/json'}
        auth = requests.auth.HTTPBasicAuth(self.username, self.password)
        resp = requests.get(url, headers=headers, verify=False, auth=auth)
        code = resp.status_code
        if code == 200:
            logger.debug('mq_work.app_image', 'query {} result:{}'.format(url, resp.json()))
            return True
        else:
            # 创建空间
            payload = {'name': str(image_name), 'shortDescription': '', 'longDescription': '', 'visibility': 'public'}
            url = 'https://' + self.host + '/' + repositories_url
            respp = requests.post(url, headers=headers, verify=False,
                                  auth=auth, data=json.dumps(payload))
            if respp.status_code == 201:
                logger.debug('mq_work.app_image', 'create repos namespace, result:{}'.format(respp.json()))
            else:
                logger.error('mq_work.app_image', 'result code:{}, msg:{}'.format(respp.status_code, self.repo_error[respp.status_code]))
            return False

    def check_image(self, hub_image, tag_name, namespace=None):
        headers = {'content-type': 'application/json'}
        if not namespace:
            namespace = self.namespace
        image_check = '/api/v0/repositories/' + namespace + '/{reponame}/tags'

        url = 'https://' + self.host + '/' + image_check.format(reponame=hub_image)
        auth = requests.auth.HTTPBasicAuth(self.username, self.password)
        resp = requests.get(url, headers=headers, verify=False, auth=auth)
        code = resp.status_code
        if code == requests.codes.ok:
            #
            jsondata = resp.json()
            tags = jsondata['tags']
            namearray = [x['name'] for x in tags]
            if tag_name in namearray:
                return True
            else:
                return False
        else:
            return False

    def check_repositories(self, repo, namespace=None):
        """ 创建repositories """
        if not namespace:
            namespace = self.namespace
        repositories_url = '/api/v0/repositories/%s' % namespace
        url = 'https://' + self.host + '/' + repositories_url + '/' + repo
        headers = {'content-type': 'application/json'}
        auth = requests.auth.HTTPBasicAuth(self.username, self.password)
        resp = requests.get(url, headers=headers, verify=False, auth=auth)
        code = resp.status_code
        repository = {}
        if code == 200:
            print resp.json()
            jsondata = resp.json()
            repository['id'] = jsondata['id']
            repository['namespace'] = jsondata['namespace']
            repository['namespaceType'] = jsondata['namespaceType']
            repository['name'] = jsondata['name']
            repository['visibility'] = jsondata['visibility']
            repository['status'] = jsondata['status']
            repository['code'] = 200
            return repository
        else:
            payload = {'name': str(repo), 'shortDescription': '', 'longDescription': '', 'visibility': 'public'}
            url = 'https://' + self.host + '/' + repositories_url
            respp = requests.post(url,
                                  headers=headers, verify=False,
                                  auth=auth, data=json.dumps(payload))
            if respp.status_code == 201:
                print respp
                print respp.json()
                jsondata = respp.json()
                repository['id'] = jsondata['id']
                repository['namespace'] = jsondata['namespace']
                repository['namespaceType'] = jsondata['namespaceType']
                repository['name'] = jsondata['name']
                repository['visibility'] = jsondata['visibility']
                repository['status'] = jsondata['status']
                repository['code'] = 200
            else:
                repository['code'] = respp.status_code
                repository['msg'] = self.repo_error[respp.status_code]
            return repository


# 命令行
if __name__ == "__main__":
    print 'aaa'
