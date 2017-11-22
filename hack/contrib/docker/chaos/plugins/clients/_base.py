# -*- coding: utf8 -*-
import socket
import json
import httplib
import httplib2
from urlparse import urlparse
import logging

from addict import Dict

logger = logging.getLogger('default')


def parse_url(url):
    if not url.startswith('http'):
        url = 'http://{}'.format(url)
    p = urlparse(url)
    items = p.netloc.split(':')

    if len(items) == 2:
        host = items[0]
        port = int(items[1])
    else:
        host = items[0]
        port = 443 if p.scheme == 'https' else 80

    info = Dict()
    info.scheme = p.scheme
    info.host = host
    info.port = port
    info.path = p.path

    return info


class Response(dict):

    """Is this response from our local cache"""
    fromcache = False

    version = 11
    status = 200
    reason = "Ok"

    previous = None

    def __init__(self, info):
        if isinstance(info, httplib.HTTPResponse):
            for key, value in info.getheaders():
                self[key.lower()] = value
            self.status = info.status
            self['status'] = str(self.status)
            self.reason = info.reason
            self.version = info.version

    def __getattr__(self, name):
        if name == 'dict':
            return self
        else:
            raise AttributeError(name)


class SuperHttpClient(object):

    class CallApiError(Exception):

        def __init__(self, apitype, url, method, res, body, describe=None):
            self.message = {
                "apitype": apitype,
                "url": url,
                "method": method,
                "httpcode": res.status,
                "body": body,
            }
            self.status = res.status

        def __str__(self):
            return json.dumps(self.message)

    class ApiSocketError(CallApiError):
        pass

    scheme = 'http'
    port = 80
    base_url = ''
    apitype = 'not design'

    def __init__(self, endpoint, timeout=25, raise_error_code=True, log_request=True, retry_count=2):
        parsed = parse_url(endpoint)
        self.host = parsed.host

        if parsed.scheme == 'https':
            self.scheme = 'https'

        if bool(parsed.port):
            self.port = parsed.port
            if parsed.port == 443:
                self.scheme = 'https'

        if bool(parsed.path):
            self.base_url = parsed.path

        self.timeout = timeout
        self.raise_error_code = raise_error_code
        self.log_request = log_request
        self.retry_count = retry_count

    def get_connection(self, *args, **kwargs):
        if self.scheme == 'https':
            conn = httplib.HTTPSConnection(self.host, self.port, timeout=self.timeout)
        else:
            conn = httplib.HTTPConnection(self.host, self.port, timeout=self.timeout)

        return conn

    def _jsondecode(self, string):
        try:
            pybody = json.loads(string)
        except ValueError:
            pybody = {"raw": string}

        return pybody

    def do_log(self, url, method, body, response, content):
        if int(response['content-length']) > 1000:
            record_content = '%s  .....ignore.....' % content[:1000]
        else:
            record_content = content

        if body is not None and len(body) > 1000:
            record_body = '%s .....ignore.....' % body[:1000]
        else:
            record_body = body

        logger.debug('request', '''{0} "{1}" body={2} response: {3} ------------- and content is {4}'''.format(method, url, record_body, response, record_content))

    def _request(self, url, method, headers={}, body=None):
        retry_count = self.retry_count

        while retry_count:
            try:
                conn = self.get_connection()
                conn.request(method, url, headers=headers, body=body)
                res = conn.getresponse()

                response = Response(res)
                content = res.read()

                try:
                    if res.status / 100 == 2:
                        if self.log_request:
                            self.do_log(url, method, body, response, content)
                    else:
                        self.do_log(url, method, body, response, content)
                except Exception, e:
                    logger.error("request", e)

                if response['content-type'].startswith('application/json'):
                    content = self._jsondecode(content)
                    if isinstance(content, dict):
                        content = Dict(content)

                if res.status / 100 != 2 and self.raise_error_code:
                    raise self.CallApiError(self.apitype, url, method, res, body)
                return response, content
            except (socket.error, socket.timeout), e:
                logger.exception('client_error', e)
                retry_count -= 1
                if retry_count:
                    logger.error("client_error", "retry request: %s" % url)
                else:
                    raise self.ApiSocketError(self.apitype, url, method, Dict({"status": 101}), {"type": "connect error", "error": str(e)})

    def _get(self, url, headers={}):
        response, content = self._request(url, 'GET', headers=headers)
        return response, content

    def _post(self, url, headers={}, body=None):
        response, content = self._request(url, 'POST', headers=headers, body=body)
        return response, content

    def _put(self, url, headers={}, body=None):
        response, content = self._request(url, 'PUT', headers=headers, body=body)
        return response, content

    def _delete(self, url, headers={}, body=None):
        response, content = self._request(url, 'DELETE', headers=headers, body=body)
        return response, content


class BaseHttpClient(object):

    class CallApiError(Exception):

        def __init__(self, apitype, url, method, res, body, describe=None):
            self.message = {
                "apitype": apitype,
                "url": url,
                "method": method,
                "httpcode": res.status,
                "body": body,
            }
            self.status = res.status

        def __str__(self):
            return json.dumps(self.message)

    class ApiSocketError(CallApiError):
        pass

    def __init__(self, *args, **kwargs):
        self.apitype = 'Not specified'
        #self.report = Dict({"ok":True})

    def _jsondecode(self, string):
        try:
            pybody = json.loads(string)
        except ValueError:
            pybody = {"raw": string}

        return pybody

    def _check_status(self, url, method, response, content):
        res = Dict(response)
        res.status = int(res.status)
        body = self._jsondecode(content)
        if isinstance(body, dict):
            body = Dict(body)
        if 400 <= res.status <= 600:
            raise self.CallApiError(self.apitype, url, method, res, body)
        else:
            return res, body

    def _request(self, url, method, headers=None, body=None):
        try:
            http = httplib2.Http(timeout=25)
            if body is None:
                response, content = http.request(url, method, headers=headers)
            else:
                response, content = http.request(url, method, headers=headers, body=body)

            if len(content) > 1000:
                record_content = '%s  .....ignore.....' % content[:1000]
            else:
                record_content = content

            if body is not None and len(body) > 1000:
                record_body = '%s .....ignore.....' % body[:1000]
            else:
                record_body = body

            logger.debug(
                'request', '''{0} "{1}" body={2} response: {3} \nand content is {4}'''.format(method, url, record_body, response, record_content))
            return response, content
        except socket.timeout, e:
            logger.exception('client_error', e)
            raise self.CallApiError(self.apitype, url, method, Dict({"status": 101}), {"type": "request time out", "error": str(e)})
        except socket.error, e:
            logger.exception('client_error', e)
            raise self.ApiSocketError(self.apitype, url, method, Dict({"status": 101}), {"type": "connect error", "error": str(e)})

    def _get(self, url, headers):
        response, content = self._request(url, 'GET', headers=headers)
        res, body = self._check_status(url, 'GET', response, content)
        return res, body

    def _post(self, url, headers, body=None):
        if body is not None:
            response, content = self._request(url, 'POST', headers=headers, body=body)
        else:
            response, content = self._request(url, 'POST', headers=headers)
        res, body = self._check_status(url, 'POST', response, content)
        return res, body

    def _put(self, url, headers, body=None):
        if body is not None:
            response, content = self._request(url, 'PUT', headers=headers, body=body)
        else:
            response, content = self._request(url, 'PUT', headers=headers)
        res, body = self._check_status(url, 'PUT', response, content)
        return res, body

    def _delete(self, url, headers, body=None):
        if body is not None:
            response, content = self._request(url, 'DELETE', headers=headers, body=body)
        else:
            response, content = self._request(url, 'DELETE', headers=headers)
        res, body = self._check_status(url, 'DELETE', response, content)
        return res, body
