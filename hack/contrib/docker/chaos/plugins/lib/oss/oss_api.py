#!/usr/bin/env python
# coding=utf-8

# Copyright (C) 2011, Alibaba Cloud Computing

# Permission is hereby granted, free of charge, to any person obtaining a
# copy of this software and associated documentation files (the
# "Software"), to deal in the Software without restriction, including
# without limitation the rights to use, copy, modify, merge, publish,
# distribute, sublicense, and/or sell copies of the Software, and to
# permit persons to whom the Software is furnished to do so, subject to
# the following conditions:

# The above copyright notice and this permission notice shall be included in all copies or substantial portions of the Software.

# THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS
# OR IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF
# MERCHANTABILITY, FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT.
# IN NO EVENT SHALL THE AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY
# CLAIM, DAMAGES OR OTHER LIABILITY, WHETHER IN AN ACTION OF CONTRACT,
# TORT OR OTHERWISE, ARISING FROM, OUT OF OR IN CONNECTION WITH THE
# SOFTWARE OR THE USE OR OTHER DEALINGS IN THE SOFTWARE.

import httplib
try:
    from oss.oss_util import *
except:
    from oss_util import *
try:
    from oss.oss_xml_handler import *
except:
    from oss_xml_handler import *


class OssAPI:
    '''
    A simple OSS API
    '''
    DefaultContentType = 'application/octet-stream'
    provider = PROVIDER
    __version__ = '0.4.2'
    Version = __version__
    AGENT = 'aliyun-sdk-python/%s (%s/%s/%s;%s)' % (__version__, platform.system(), platform.release(), platform.machine(), platform.python_version())

    def __init__(self, host='oss.aliyuncs.com', access_id='', secret_access_key='', port=80, is_security=False, sts_token=None):
        self.SendBufferSize = 8192
        self.RecvBufferSize = 1024 * 1024 * 10
        self.host = get_host_from_list(host)
        self.port = port
        self.access_id = access_id
        self.secret_access_key = secret_access_key
        self.show_bar = False
        self.is_security = is_security
        self.retry_times = 5
        self.agent = self.AGENT
        self.debug = False
        self.timeout = 60
        self.is_oss_domain = False
        self.sts_token = sts_token

    def set_timeout(self, timeout):
        self.timeout = timeout

    def set_debug(self, is_debug):
        if is_debug:
            self.debug = True

    def set_retry_times(self, retry_times=5):
        self.retry_times = retry_times

    def set_send_buf_size(self, buf_size):
        try:
            self.SendBufferSize = (int)(buf_size)
        except ValueError:
            pass

    def set_recv_buf_size(self, buf_size):
        try:
            self.RecvBufferSize = (int)(buf_size)
        except ValueError:
            pass

    def set_is_oss_host(self, is_oss_host=False):
        if is_oss_host:
            self.is_oss_domain = True
        else:
            self.is_oss_domain = False

    def get_connection(self, tmp_host=None):
        host = ''
        port = 80
        if not tmp_host:
            tmp_host = self.host
        host_port_list = tmp_host.split(":")
        if len(host_port_list) == 1:
            host = host_port_list[0].strip()
        elif len(host_port_list) == 2:
            host = host_port_list[0].strip()
            port = int(host_port_list[1].strip())
        if self.is_security or port == 443:
            self.is_security = True
            if sys.version_info >= (2, 6):
                return httplib.HTTPSConnection(host=host, port=port, timeout=self.timeout)
            else:
                return httplib.HTTPSConnection(host=host, port=port)
        else:
            if sys.version_info >= (2, 6):
                return httplib.HTTPConnection(host=host, port=port, timeout=self.timeout)
            else:
                return httplib.HTTPConnection(host=host, port=port)

    def sign_url_auth_with_expire_time(self, method, url, headers=None, resource="/", timeout=60, params=None):
        '''
        Create the authorization for OSS based on the input method, url, body and headers

        :type method: string
        :param method: one of PUT, GET, DELETE, HEAD

        :type url: string
        :param:HTTP address of bucket or object, eg: http://HOST/bucket/object

        :type headers: dict
        :param: HTTP header

        :type resource: string
        :param:path of bucket or object, eg: /bucket/ or /bucket/object

        :type timeout: int
        :param

        Returns:
            signature url.
        '''
        if not headers:
            headers = {}
        if not params:
            params = {}
        send_time = str(int(time.time()) + timeout)
        headers['Date'] = send_time
        auth_value = get_assign(self.secret_access_key, method, headers, resource, None, self.debug)
        params["OSSAccessKeyId"] = self.access_id
        params["Expires"] = str(send_time)
        params["Signature"] = auth_value
        sign_url = append_param(url, params)
        return sign_url

    def sign_url(self, method, bucket, object, timeout=60, headers=None, params=None):
        '''
        Create the authorization for OSS based on the input method, url, body and headers

        :type method: string
        :param method: one of PUT, GET, DELETE, HEAD

        :type bucket: string
        :param:

        :type object: string
        :param:

        :type timeout: int
        :param

        :type headers: dict
        :param: HTTP header

        :type params: dict
        :param: the parameters that put in the url address as query string

        :type resource: string
        :param:path of bucket or object, eg: /bucket/ or /bucket/object

        Returns:
            signature url.
        '''
        if not headers:
            headers = {}
        if not params:
            params = {}
        send_time = str(int(time.time()) + timeout)
        headers['Date'] = send_time
        object = convert_utf8(object)
        resource = "/%s/%s%s" % (bucket, object, get_resource(params))
        auth_value = get_assign(self.secret_access_key, method, headers, resource, None, self.debug)
        params["OSSAccessKeyId"] = self.access_id
        params["Expires"] = str(send_time)
        params["Signature"] = auth_value
        url = ''
        object = oss_quote(object)
        http = "http"
        if self.is_security:
            http = "https"
        if is_ip(self.host):
            url = "%s://%s/%s/%s" % (http, self.host, bucket, object)
        elif is_oss_host(self.host, self.is_oss_domain):
            if check_bucket_valid(bucket):
                url = "%s://%s.%s/%s" % (http, bucket, self.host, object)
            else:
                url = "%s://%s/%s/%s" % (http, self.host, bucket, object)
        else:
            url = "%s://%s/%s" % (http, self.host, object)
        sign_url = append_param(url, params)
        return sign_url

    def _create_sign_for_normal_auth(self, method, headers=None, resource="/"):
        '''
        NOT public API
        Create the authorization for OSS based on header input.
        it should be put into "Authorization" parameter of header.

        :type method: string
        :param:one of PUT, GET, DELETE, HEAD

        :type headers: dict
        :param: HTTP header

        :type resource: string
        :param:path of bucket or object, eg: /bucket/ or /bucket/object

        Returns:
            signature string
        '''
        auth_value = "%s %s:%s" % (self.provider, self.access_id, get_assign(self.secret_access_key, method, headers, resource, None, self.debug))
        return auth_value

    def bucket_operation(self, method, bucket, headers=None, params=None):
        return self.http_request(method, bucket, '', headers, '', params)

    def object_operation(self, method, bucket, object, headers=None, body='', params=None):
        return self.http_request(method, bucket, object, headers, body, params)

    def http_request(self, method, bucket, object, headers=None, body='', params=None):
        '''
        Send http request of operation

        :type method: string
        :param method: one of PUT, GET, DELETE, HEAD, POST

        :type bucket: string
        :param

        :type object: string
        :param

        :type headers: dict
        :param: HTTP header

        :type body: string
        :param

        Returns:
            HTTP Response
        '''
        retry = 5
        res = None
        while retry > 0:
            retry -= 1
            tmp_bucket = bucket
            tmp_object = object
            tmp_headers = {}
            if headers and isinstance(headers, dict):
                tmp_headers = headers.copy()
            tmp_params = {}
            if params and isinstance(params, dict):
                tmp_params = params.copy()

            res = self.http_request_with_redirect(method, tmp_bucket, tmp_object, tmp_headers, body, tmp_params)
            if check_redirect(res):
                self.host = helper_get_host_from_resp(res, bucket)
            else:
                return res
        return res

    def http_request_with_redirect(self, method, bucket, object, headers=None, body='', params=None):
        '''
        Send http request of operation

        :type method: string
        :param method: one of PUT, GET, DELETE, HEAD, POST

        :type bucket: string
        :param

        :type object: string
        :param

        :type headers: dict
        :param: HTTP header

        :type body: string
        :param

        Returns:
            HTTP Response
        '''
        if not params:
            params = {}
        if not headers:
            headers = {}
        if self.sts_token:
            headers['x-oss-security-token'] = self.sts_token
        object = convert_utf8(object)
        if not bucket:
            resource = "/"
            headers['Host'] = self.host
        else:
            headers['Host'] = "%s.%s" % (bucket, self.host)
            if not is_oss_host(self.host, self.is_oss_domain):
                headers['Host'] = self.host
            resource = "/%s/" % bucket
        resource = convert_utf8(resource)
        resource = "%s%s%s" % (resource, object, get_resource(params))
        object = oss_quote(object)
        url = "/%s" % object
        if is_ip(self.host):
            url = "/%s/%s" % (bucket, object)
            if not bucket:
                url = "/%s" % object
            headers['Host'] = self.host
        url = append_param(url, params)
        date = time.strftime("%a, %d %b %Y %H:%M:%S GMT", time.gmtime())
        headers['Date'] = date
        headers['Authorization'] = self._create_sign_for_normal_auth(method, headers, resource)
        headers['User-Agent'] = self.agent
        if check_bucket_valid(bucket) and not is_ip(self.host):
            conn = self.get_connection(headers['Host'])
        else:
            conn = self.get_connection()
        conn.request(method, url, body, headers)
        return conn.getresponse()

    def get_service(self, headers=None, prefix='', marker='', maxKeys=''):
        '''
        List all buckets of user
        '''
        return self.list_all_my_buckets(headers, prefix, marker, maxKeys)

    def list_all_my_buckets(self, headers=None, prefix='', marker='', maxKeys=''):
        '''
        List all buckets of user
        type headers: dict
        :param

        Returns:
            HTTP Response
        '''
        method = 'GET'
        bucket = ''
        object = ''
        body = ''
        params = {}
        if prefix != '':
            params['prefix'] = prefix
        if marker != '':
            params['marker'] = marker
        if maxKeys != '':
            params['max-keys'] = maxKeys
        return self.http_request(method, bucket, object, headers, body, params)

    def get_bucket_acl(self, bucket):
        '''
        Get Access Control Level of bucket

        :type bucket: string
        :param

        Returns:
            HTTP Response
        '''
        method = 'GET'
        object = ''
        headers = {}
        body = ''
        params = {}
        params['acl'] = ''
        return self.http_request(method, bucket, object, headers, body, params)

    def get_bucket_location(self, bucket):
        '''
        Get Location of bucket
        '''
        method = 'GET'
        object = ''
        headers = {}
        body = ''
        params = {}
        params['location'] = ''
        return self.http_request(method, bucket, object, headers, body, params)

    def get_bucket(self, bucket, prefix='', marker='', delimiter='', maxkeys='', headers=None, encoding_type=''):
        '''
        List object that in bucket
        '''
        return self.list_bucket(bucket, prefix, marker, delimiter, maxkeys, headers, encoding_type)

    def list_bucket(self, bucket, prefix='', marker='', delimiter='', maxkeys='', headers=None, encoding_type=''):
        '''
        List object that in bucket

        :type bucket: string
        :param

        :type prefix: string
        :param

        :type marker: string
        :param

        :type delimiter: string
        :param

        :type maxkeys: string
        :param

        :type headers: dict
        :param: HTTP header

        :type maxkeys: string
        :encoding_type

        Returns:
            HTTP Response
        '''
        method = 'GET'
        object = ''
        body = ''
        params = {}
        params['prefix'] = prefix
        params['marker'] = marker
        params['delimiter'] = delimiter
        params['max-keys'] = maxkeys
        params['encoding-type'] = encoding_type
        return self.http_request(method, bucket, object, headers, body, params)

    def get_website(self, bucket, headers=None):
        '''
        Get bucket website

        :type bucket: string
        :param

        Returns:
            HTTP Response
        '''
        method = 'GET'
        object = ''
        body = ''
        params = {}
        params['website'] = ''
        return self.http_request(method, bucket, object, headers, body, params)

    def get_lifecycle(self, bucket, headers=None):
        '''
        Get bucket lifecycle

        :type bucket: string
        :param

        Returns:
            HTTP Response
        '''
        method = 'GET'
        object = ''
        body = ''
        params = {}
        params['lifecycle'] = ''
        return self.http_request(method, bucket, object, headers, body, params)

    def get_logging(self, bucket, headers=None):
        '''
        Get bucket logging

        :type bucket: string
        :param

        Returns:
            HTTP Response
        '''
        method = 'GET'
        object = ''
        body = ''
        params = {}
        params['logging'] = ''
        return self.http_request(method, bucket, object, headers, body, params)

    def get_cors(self, bucket, headers=None):
        '''
        Get bucket cors

        :type bucket: string
        :param

        Returns:
            HTTP Response
        '''
        method = 'GET'
        object = ''
        body = ''
        params = {}
        params['cors'] = ''
        return self.http_request(method, bucket, object, headers, body, params)

    def create_bucket(self, bucket, acl='', headers=None):
        '''
        Create bucket
        '''
        return self.put_bucket(bucket, acl, headers)

    def put_bucket(self, bucket, acl='', headers=None):
        '''
        Create bucket

        :type bucket: string
        :param

        :type acl: string
        :param: one of private public-read public-read-write

        :type headers: dict
        :param: HTTP header

        Returns:
            HTTP Response
        '''
        if not headers:
            headers = {}
        if acl != '':
            if "AWS" == self.provider:
                headers['x-amz-acl'] = acl
            else:
                headers['x-oss-acl'] = acl
        method = 'PUT'
        object = ''
        body = ''
        params = {}
        return self.http_request(method, bucket, object, headers, body, params)

    def put_logging(self, sourcebucket, targetbucket, prefix):
        '''
        Put bucket logging

        :type sourcebucket: string
        :param 

        :type targetbucket: string
        :param: Specifies the bucket where you want Aliyun OSS to store server access logs

        :type prefix: string
        :param: This element lets you specify a prefix for the objects that the log files will be stored

        Returns:
            HTTP Response
        '''
        body = '<BucketLoggingStatus>'
        if targetbucket:
            body += '<LoggingEnabled>'
            body += '<TargetBucket>%s</TargetBucket>' % convert_utf8(targetbucket)
            if prefix:
                body += '<TargetPrefix>%s</TargetPrefix>' % convert_utf8(prefix)
            body += '</LoggingEnabled>'
        body += '</BucketLoggingStatus>'
        method = 'PUT'
        object = ''
        params = {}
        headers = {}
        params['logging'] = ''
        return self.http_request(method, sourcebucket, object, headers, body, params)

    def put_website(self, bucket, indexfile, errorfile):
        '''
        Put bucket website

        :type bucket: string
        :param

        :type indexfile: string
        :param: the object that contain index page

        :type errorfile: string
        :param: the object taht contain error page

        Returns:
            HTTP Response
        '''
        indexfile = convert_utf8(indexfile)
        errorfile = convert_utf8(errorfile)
        body = '<WebsiteConfiguration><IndexDocument><Suffix>%s</Suffix></IndexDocument><ErrorDocument><Key>%s</Key></ErrorDocument></WebsiteConfiguration>' % (
            indexfile, errorfile)
        method = 'PUT'
        object = ''
        headers = {}
        params = {}
        params['website'] = ''
        return self.http_request(method, bucket, object, headers, body, params)

    def put_lifecycle(self, bucket, lifecycle):
        '''
        Put bucket lifecycle

        :type bucket: string
        :param

        :type lifecycle: string
        :param: lifecycle configuration

        Returns:
            HTTP Response
        '''
        body = lifecycle
        method = 'PUT'
        object = ''
        headers = {}
        params = {}
        params['lifecycle'] = ''
        return self.http_request(method, bucket, object, headers, body, params)

    def put_cors(self, bucket, cors_xml, headers=None):
        '''
        Put bucket cors

        :type bucket: string
        :param

        :type cors_xml: string
        :param: the xml that contain cors rules 

        Returns:
            HTTP Response
        '''
        body = cors_xml
        method = 'PUT'
        object = ''
        if not headers:
            headers = {}
        headers['Content-Length'] = str(len(body))
        base64md5 = get_string_base64_md5(body)
        headers['Content-MD5'] = base64md5
        params = {}
        params['cors'] = ''
        return self.http_request(method, bucket, object, headers, body, params)

    def put_bucket_with_location(self, bucket, acl='', location='', headers=None):
        '''
        Create bucket

        :type bucket: string
        :param

        :type acl: string
        :param: one of private public-read public-read-write

        :type location: string
        :param:

        :type headers: dict
        :param: HTTP header

        Returns:
            HTTP Response
        '''
        if not headers:
            headers = {}
        if acl != '':
            if "AWS" == self.provider:
                headers['x-amz-acl'] = acl
            else:
                headers['x-oss-acl'] = acl
        params = {}
        body = ''
        if location != '':
            body = r'<CreateBucketConfiguration>'
            body += r'<LocationConstraint>'
            body += location
            body += r'</LocationConstraint>'
            body += r'</CreateBucketConfiguration>'
        method = 'PUT'
        object = ''
        return self.http_request(method, bucket, object, headers, body, params)

    def delete_bucket(self, bucket, headers=None):
        '''
        Delete bucket

        :type bucket: string
        :param

        Returns:
            HTTP Response
        '''
        method = 'DELETE'
        object = ''
        body = ''
        params = {}
        return self.http_request(method, bucket, object, headers, body, params)

    def delete_website(self, bucket, headers=None):
        '''
        Delete bucket website

        :type bucket: string
        :param

        Returns:
            HTTP Response
        '''
        method = 'DELETE'
        object = ''
        body = ''
        params = {}
        params['website'] = ''
        return self.http_request(method, bucket, object, headers, body, params)

    def delete_lifecycle(self, bucket, headers=None):
        '''
        Delete bucket lifecycle

        :type bucket: string
        :param

        Returns:
            HTTP Response
        '''
        method = 'DELETE'
        object = ''
        body = ''
        params = {}
        params['lifecycle'] = ''
        return self.http_request(method, bucket, object, headers, body, params)

    def delete_logging(self, bucket, headers=None):
        '''
        Delete bucket logging

        :type bucket: string
        :param:

        Returns:
            HTTP Response
        '''
        method = 'DELETE'
        object = ''
        body = ''
        params = {}
        params['logging'] = ''
        return self.http_request(method, bucket, object, headers, body, params)

    def delete_cors(self, bucket, headers=None):
        '''
        Delete bucket cors 

        :type bucket: string
        :param:

        Returns:
            HTTP Response
        '''
        method = 'DELETE'
        object = ''
        body = ''
        params = {}
        params['cors'] = ''
        return self.http_request(method, bucket, object, headers, body, params)

    def put_object_with_data(self, bucket, object, input_content, content_type='', headers=None, params=None):
        '''
        Put object into bucket, the content of object is from input_content
        '''
        return self.put_object_from_string(bucket, object, input_content, content_type, headers, params)

    def put_object_from_string(self, bucket, object, input_content, content_type='', headers=None, params=None):
        '''
        Put object into bucket, the content of object is from input_content

        :type bucket: string
        :param

        :type object: string
        :param

        :type input_content: string
        :param

        :type content_type: string
        :param: the object content type that supported by HTTP

        :type headers: dict
        :param: HTTP header

        Returns:
            HTTP Response
        '''
        method = "PUT"
        return self._put_or_post_object_from_string(method, bucket, object, input_content, content_type, headers, params)

    def post_object_from_string(self, bucket, object, input_content, content_type='', headers=None, params=None):
        '''
        Post object into bucket, the content of object is from input_content

        :type bucket: string
        :param

        :type object: string
        :param

        :type input_content: string
        :param

        :type content_type: string
        :param: the object content type that supported by HTTP

        :type headers: dict
        :param: HTTP header

        Returns:
            HTTP Response
        '''
        method = "POST"
        return self._put_or_post_object_from_string(method, bucket, object, input_content, content_type, headers, params)

    def _put_or_post_object_from_string(self, method, bucket, object, input_content, content_type, headers, params):
        if not headers:
            headers = {}
        if not content_type:
            content_type = get_content_type_by_filename(object)
        if not headers.has_key('Content-Type') and not headers.has_key('content-type'):
            headers['Content-Type'] = content_type
        headers['Content-Length'] = str(len(input_content))
        fp = StringIO.StringIO(input_content)
        if "POST" == method:
            res = self.post_object_from_fp(bucket, object, fp, content_type, headers, params)
        else:
            res = self.put_object_from_fp(bucket, object, fp, content_type, headers, params)
        fp.close()
        return res

    def _open_conn_to_put_object(self, method, bucket, object, filesize, content_type=DefaultContentType, headers=None, params=None):
        '''
        NOT public API
        Open a connectioon to put object

        :type bucket: string
        :param

        :type filesize: int
        :param

        :type object: string
        :param

        :type input_content: string
        :param

        :type content_type: string
        :param: the object content type that supported by HTTP

        :type headers: dict
        :param: HTTP header

        Returns:
            Initialized HTTPConnection
        '''
        if not params:
            params = {}
        if not headers:
            headers = {}
        if self.sts_token:
            headers['x-oss-security-token'] = self.sts_token
        object = convert_utf8(object)
        resource = "/%s/" % bucket
        if not bucket:
            resource = "/"
        resource = convert_utf8(resource)
        resource = "%s%s%s" % (resource, object, get_resource(params))

        object = oss_quote(object)
        url = "/%s" % object
        if bucket:
            headers['Host'] = "%s.%s" % (bucket, self.host)
            if not is_oss_host(self.host, self.is_oss_domain):
                headers['Host'] = self.host
        else:
            headers['Host'] = self.host
        if is_ip(self.host):
            url = "/%s/%s" % (bucket, object)
            headers['Host'] = self.host
        url = append_param(url, params)
        date = time.strftime("%a, %d %b %Y %H:%M:%S GMT", time.gmtime())

        if check_bucket_valid(bucket) and not is_ip(self.host):
            conn = self.get_connection(headers['Host'])
        else:
            conn = self.get_connection()
        conn.putrequest(method, url)
        content_type = convert_utf8(content_type)
        if not headers.has_key('Content-Type') and not headers.has_key('content-type'):
            headers['Content-Type'] = content_type
        headers["Content-Length"] = filesize
        headers["Date"] = date
        headers["Expect"] = "100-Continue"
        headers['User-Agent'] = self.agent
        for k in headers.keys():
            conn.putheader(str(k), str(headers[k]))
        if '' != self.secret_access_key and '' != self.access_id:
            auth = self._create_sign_for_normal_auth(method, headers, resource)
            conn.putheader("Authorization", auth)
        conn.endheaders()
        return conn

    def put_object_from_file(self, bucket, object, filename, content_type='', headers=None, params=None):
        '''
        put object into bucket, the content of object is read from file

        :type bucket: string
        :param

        :type object: string
        :param

        :type fllename: string
        :param: the name of the read file

        :type content_type: string
        :param: the object content type that supported by HTTP

        :type headers: dict
        :param: HTTP header

        Returns:
            HTTP Response
        '''
        fp = open(filename, 'rb')
        if not content_type:
            content_type = get_content_type_by_filename(filename)
        res = self.put_object_from_fp(bucket, object, fp, content_type, headers, params)
        fp.close()
        return res

    def append_object_from_string(self, bucket, object, position, content, content_type='', headers=None):
        '''
        Append content into object, the append content of object is from input_content

        :type bucket: string
        :param

        :type object: string
        :param

        :type position: int
        :param: append start position

        :type input_content: string
        :param

        :type content_type: string
        :param: the object content type that supported by HTTP

        :type headers: dict
        :param: HTTP header

        Returns:
            HTTP Response
        '''
        if not headers:
            headers = {}
        if not content_type:
            content_type = get_content_type_by_filename(object)
        if not headers.has_key('Content-Type') and not headers.has_key('content-type'):
            headers['Content-Type'] = content_type
        headers['Content-Length'] = str(len(content))

        params = {}
        params['append'] = ''
        params['position'] = str(position)

        method = 'POST'
        conn = self._open_conn_to_put_object(method, bucket, object, len(content), content_type, headers, params)
        conn.send(content)

        return conn.getresponse()

    def append_object_from_file(self, bucket, object, position, filename, content_type='', headers=None):
        '''
        Append content into object, the content of object is read from file

        :type bucket: string
        :param

        :type object: string
        :param

        :type position: int
        :param: append start position

        :type fllename: string
        :param: the name of the read file

        :type content_type: string
        :param: the object content type that supported by HTTP

        :type headers: dict
        :param: HTTP header

        Returns:
            HTTP Response
        '''
        params = {}
        params['append'] = ''
        params['position'] = str(position)

        if not headers:
            headers = {}
        if not content_type:
            content_type = get_content_type_by_filename(object)
        if not headers.has_key('Content-Type') and not headers.has_key('content-type'):
            headers['Content-Type'] = content_type
        fp = open(filename, 'rb')
        res = self.post_object_from_fp(bucket, object, fp, content_type, headers, params)
        fp.close()
        return res

    def post_object_from_file(self, bucket, object, filename, content_type='', headers=None, params=None):
        '''
        post object into bucket, the content of object is read from file

        :type bucket: string
        :param

        :type object: string
        :param

        :type fllename: string
        :param: the name of the read file

        :type content_type: string
        :param: the object content type that supported by HTTP

        :type headers: dict
        :param: HTTP header

        Returns:
            HTTP Response
        '''
        fp = open(filename, 'rb')
        if not content_type:
            content_type = get_content_type_by_filename(filename)
        res = self.post_object_from_fp(bucket, object, fp, content_type, headers, params)
        fp.close()
        return res

    def view_bar(self, num=1, sum=100):
        if sum != 0:
            rate = float(num) / float(sum)
            rate_num = int(rate * 100)
            print '\r%d%% ' % (rate_num),
            sys.stdout.flush()

    def put_object_from_fp(self, bucket, object, fp, content_type=DefaultContentType, headers=None, params=None):
        '''
        Put object into bucket, the content of object is read from file pointer

        :type bucket: string
        :param

        :type object: string
        :param

        :type fp: file
        :param: the pointer of the read file

        :type content_type: string
        :param: the object content type that supported by HTTP

        :type headers: dict
        :param: HTTP header

        Returns:
            HTTP Response
        '''
        method = 'PUT'
        return self._put_or_post_object_from_fp(method, bucket, object, fp, content_type, headers, params)

    def post_object_from_fp(self, bucket, object, fp, content_type=DefaultContentType, headers=None, params=None):
        '''
        Post object into bucket, the content of object is read from file pointer

        :type bucket: string
        :param

        :type object: string
        :param

        :type fp: file
        :param: the pointer of the read file

        :type content_type: string
        :param: the object content type that supported by HTTP

        :type headers: dict
        :param: HTTP header

        Returns:
            HTTP Response
        '''
        method = 'POST'
        return self._put_or_post_object_from_fp(method, bucket, object, fp, content_type, headers, params)

    def _put_or_post_object_from_fp(self, method, bucket, object, fp, content_type=DefaultContentType, headers=None, params=None):
        tmp_object = object
        tmp_headers = {}
        tmp_params = {}
        if headers and isinstance(headers, dict):
            tmp_headers = headers.copy()
        if params and isinstance(params, dict):
            tmp_params = params.copy()

        fp.seek(os.SEEK_SET, os.SEEK_END)
        filesize = fp.tell()
        fp.seek(os.SEEK_SET)
        conn = self._open_conn_to_put_object(method, bucket, object, filesize, content_type, headers, params)
        totallen = 0
        l = fp.read(self.SendBufferSize)
        retry_times = 0
        while len(l) > 0:
            if retry_times > 100:
                print "reach max retry times: %s" % retry_times
                raise
            try:
                conn.send(l)
                retry_times = 0
            except:
                retry_times += 1
                continue
            totallen += len(l)
            if self.show_bar:
                self.view_bar(totallen, filesize)
            l = fp.read(self.SendBufferSize)
        res = conn.getresponse()
        if check_redirect(res):
            self.host = helper_get_host_from_resp(res, bucket)
            return self.put_object_from_fp(bucket, tmp_object, fp, content_type, tmp_headers, tmp_params)
        return res

    def get_object(self, bucket, object, headers=None, params=None):
        '''
        Get object

        :type bucket: string
        :param

        :type object: string
        :param

        :type headers: dict
        :param: HTTP header

        Returns:
            HTTP Response
        '''
        method = 'GET'
        body = ''
        return self.http_request(method, bucket, object, headers, body, params)

    def get_object_to_file(self, bucket, object, filename, headers=None):
        '''
        Get object and write the content of object into a file

        :type bucket: string
        :param

        :type object: string
        :param

        :type filename: string
        :param

        :type headers: dict
        :param: HTTP header

        Returns:
            HTTP Response
        '''
        res = self.get_object(bucket, object, headers)
        totalread = 0
        if res.status / 100 == 2:
            header = {}
            header = convert_header2map(res.getheaders())
            filesize = safe_get_element("content-length", header)
            f = file(filename, 'wb')
            data = ''
            while True:
                data = res.read(self.RecvBufferSize)
                if data:
                    f.write(data)
                    totalread += len(data)
                    if self.show_bar:
                        self.view_bar(totalread, filesize)
                else:
                    break
            f.close()
        return res

    def delete_object(self, bucket, object, headers=None):
        '''
        Delete object

        :type bucket: string
        :param

        :type object: string
        :param

        :type headers: dict
        :param: HTTP header

        Returns:
            HTTP Response
        '''
        method = 'DELETE'
        body = ''
        params = {}
        return self.http_request(method, bucket, object, headers, body, params)

    def head_object(self, bucket, object, headers=None):
        '''
        Head object, to get the meta message of object without the content

        :type bucket: string
        :param

        :type object: string
        :param

        :type headers: dict
        :param: HTTP header

        Returns:
            HTTP Response
        '''
        method = 'HEAD'
        body = ''
        params = {}
        return self.http_request(method, bucket, object, headers, body, params)

    def create_link_from_list(self, bucket, object, object_list=None, headers=None, params=None):
        object_link_msg_xml = create_object_link_msg_xml_by_name(object_list)
        return self.create_link(bucket, object, object_link_msg_xml, headers, params)

    def create_link(self, bucket, object, object_link_msg_xml, headers=None, params=None):
        '''
        Create object link, merge all objects in object_link_msg_xml into one object
        :type bucket: string
        :param

        :type object: string
        :param

        :type object_link_msg_xml: string
        :param: xml format string, like
                <CreateObjectLink>
                    <Part>
                        <PartNumber>N</PartNumber>
                        <PartName>objectN</PartName>
                    </Part>
                </CreateObjectLink>
        :type headers: dict
        :param: HTTP header

        :type params: dict
        :param: parameters

        Returns:
            HTTP Response
        '''
        method = 'PUT'
        if not headers:
            headers = {}
        if not params:
            params = {}
        if not headers.has_key('Content-Type'):
            content_type = get_content_type_by_filename(object)
            headers['Content-Type'] = content_type
        body = object_link_msg_xml
        params['link'] = ''
        headers['Content-Length'] = str(len(body))
        return self.http_request(method, bucket, object, headers, body, params)

    def get_link_index(self, bucket, object, headers=None, params=None):
        '''
        Get all objects linked

        :type bucket: string
        :param

        :type object: string
        :param

        :type headers: dict
        :param: HTTP header

        Returns:
            HTTP Response
        '''
        method = 'GET'
        if not headers:
            headers = {}
        if not params:
            params = {}
        params['link'] = ''
        body = ''
        return self.http_request(method, bucket, object, headers, body, params)

    def post_object_group(self, bucket, object, object_group_msg_xml, headers=None, params=None):
        '''
        Post object group, merge all objects in object_group_msg_xml into one object
        :type bucket: string
        :param

        :type object: string
        :param

        :type object_group_msg_xml: string
        :param: xml format string, like
                <CreateFileGroup>
                    <Part>
                        <PartNumber>N</PartNumber>
                        <FileName>objectN</FileName>
                        <Etag>"47BCE5C74F589F4867DBD57E9CA9F808"</Etag>
                    </Part>
                </CreateFileGroup>
        :type headers: dict
        :param: HTTP header

        :type params: dict
        :param: parameters

        Returns:
            HTTP Response
        '''
        method = 'POST'
        if not headers:
            headers = {}
        if not params:
            params = {}
        if not headers.has_key('Content-Type'):
            content_type = get_content_type_by_filename(object)
            headers['Content-Type'] = content_type
        body = object_group_msg_xml
        params['group'] = ''
        headers['Content-Length'] = str(len(body))
        return self.http_request(method, bucket, object, headers, body, params)

    def get_object_group_index(self, bucket, object, headers=None):
        '''
        Get object group_index

        :type bucket: string
        :param

        :type object: string
        :param

        :type headers: dict
        :param: HTTP header

        Returns:
            HTTP Response
        '''
        if not headers:
            headers = {}
        headers["x-oss-file-group"] = ''
        method = 'GET'
        body = ''
        params = {}
        return self.http_request(method, bucket, object, headers, body, params)

    def upload_part_from_file_given_pos(self, bucket, object, filename, offset, partsize, upload_id, part_number, headers=None, params=None):
        if not params:
            params = {}
        params['partNumber'] = part_number
        params['uploadId'] = upload_id
        content_type = ''
        return self.put_object_from_file_given_pos(bucket, object, filename, offset, partsize, content_type, headers, params)

    def put_object_from_file_given_pos(self, bucket, object, filename, offset, partsize, content_type='', headers=None, params=None):
        '''
        Put object into bucket, the content of object is read from given posision of filename
        :type bucket: string
        :param

        :type object: string
        :param

        :type fllename: string
        :param: the name of the read file

        :type offset: int
        :param: the given position of file

        :type partsize: int
        :param: the size of read content

        :type content_type: string
        :param: the object content type that supported by HTTP

        :type headers: dict
        :param: HTTP header

        Returns:
            HTTP Response
        '''
        tmp_object = object
        tmp_headers = {}
        tmp_params = {}
        if headers and isinstance(headers, dict):
            tmp_headers = headers.copy()
        if params and isinstance(params, dict):
            tmp_params = params.copy()

        fp = open(filename, 'rb')
        if offset > os.path.getsize(filename):
            fp.seek(os.SEEK_SET, os.SEEK_END)
        else:
            fp.seek(offset)
        if not content_type:
            content_type = get_content_type_by_filename(filename)
        method = 'PUT'
        conn = self._open_conn_to_put_object(method, bucket, object, partsize, content_type, headers, params)
        left_len = partsize
        while 1:
            if left_len <= 0:
                break
            elif left_len < self.SendBufferSize:
                buffer_content = fp.read(left_len)
            else:
                buffer_content = fp.read(self.SendBufferSize)
            if buffer_content:
                retry_times = 0
                while 1:
                    if retry_times > 100:
                        print "reach max retry times: %s" % retry_times
                        fp.close()
                        raise
                    try:
                        conn.send(buffer_content)
                        retry_times = 0
                        break
                    except:
                        retry_times += 1
                        continue
            left_len = left_len - len(buffer_content)

        fp.close()
        res = conn.getresponse()
        if check_redirect(res):
            self.host = helper_get_host_from_resp(res, bucket)
            return self.put_object_from_file_given_pos(bucket, tmp_object, filename, offset, partsize, content_type, tmp_headers, tmp_params)
        return res

    def upload_large_file(self, bucket, object, filename, thread_num=10, max_part_num=1000, headers=None):
        '''
        Upload large file, the content is read from filename. 
        The large file is splitted into many parts. It will put the many parts into bucket 
        and then merge all the parts into one object.

        :type bucket: string
        :param

        :type object: string
        :param

        :type fllename: string
        :param: the name of the read file

        :type thread_num: int
        :param

        :type max_part_num: int
        :param

        :type headers: dict
        :param

        Returns:
            HTTP Response

        '''
        # split the large file into 1000 parts or many parts
        # get part_msg_list
        if not headers:
            headers = {}
        filename = convert_utf8(filename)
        part_msg_list = split_large_file(filename, object, max_part_num)
        # make sure all the parts are put into same bucket
        if len(part_msg_list) < thread_num and len(part_msg_list) != 0:
            thread_num = len(part_msg_list)
        step = len(part_msg_list) / thread_num
        retry_times = self.retry_times
        while(retry_times >= 0):
            try:
                threadpool = []
                for i in xrange(0, thread_num):
                    if i == thread_num - 1:
                        end = len(part_msg_list)
                    else:
                        end = i * step + step
                    begin = i * step
                    oss = OssAPI(self.host, self.access_id, self.secret_access_key)
                    current = PutObjectGroupWorker(oss, bucket, filename, part_msg_list[begin:end], retry_times)
                    threadpool.append(current)
                    current.start()
                for item in threadpool:
                    item.join()
                break
            except:
                retry_times = retry_times - 1
        if -1 >= retry_times:
            print "after retry %s, failed, upload large file failed!" % retry_times
            return
        # get xml string that contains msg of object group
        object_group_msg_xml = create_object_group_msg_xml(part_msg_list)
        content_type = get_content_type_by_filename(filename)
        content_type = convert_utf8(content_type)
        if not headers.has_key('Content-Type'):
            headers['Content-Type'] = content_type
        return self.post_object_group(bucket, object, object_group_msg_xml, headers)

    def upload_large_file_by_link(self, bucket, object, filename, thread_num=5, max_part_num=50, headers=None):
        '''
        Upload large file, the content is read from filename. The large file is splitted into many parts. 
        all the parts are put into bucket and then merged into one object.

        :type bucket: string
        :param

        :type object: string
        :param

        :type fllename: string
        :param: the name of the read file

        :type thread_num: int
        :param

        :type max_part_num: int
        :param

        :type headers: dict
        :param

        Returns:
            HTTP Response

        '''
        # split the large file into 100 parts or many parts
        # get part_msg_list
        if not headers:
            headers = {}
        filename = convert_utf8(filename)
        part_msg_list = split_large_file(filename, object, max_part_num)
        # make sure all the parts are put into same bucket
        if len(part_msg_list) < thread_num and len(part_msg_list) != 0:
            thread_num = len(part_msg_list)
        step = len(part_msg_list) / thread_num
        retry_times = self.retry_times
        while(retry_times >= 0):
            try:
                threadpool = []
                for i in xrange(0, thread_num):
                    if i == thread_num - 1:
                        end = len(part_msg_list)
                    else:
                        end = i * step + step
                    begin = i * step
                    oss = OssAPI(self.host, self.access_id, self.secret_access_key)
                    current = PutObjectLinkWorker(oss, bucket, filename, part_msg_list[begin:end], self.retry_times)
                    threadpool.append(current)
                    current.start()
                for item in threadpool:
                    item.join()
                break
            except:
                retry_times = retry_times - 1
        if -1 >= retry_times:
            print "after retry %s, failed, upload large file failed!" % retry_times
            return
        # get xml string that contains msg of object link
        object_link_msg_xml = create_object_link_msg_xml(part_msg_list)
        content_type = get_content_type_by_filename(filename)
        content_type = convert_utf8(content_type)
        if not headers.has_key('Content-Type'):
            headers['Content-Type'] = content_type
        return self.create_link(bucket, object, object_link_msg_xml, headers)

    def copy_object(self, source_bucket, source_object, target_bucket, target_object, headers=None):
        '''
        Copy object

        :type source_bucket: string
        :param

        :type source_object: string
        :param

        :type target_bucket: string
        :param

        :type target_object: string
        :param

        :type headers: dict
        :param: HTTP header

        Returns:
            HTTP Response
        '''
        if not headers:
            headers = {}
        source_object = convert_utf8(source_object)
        source_object = oss_quote(source_object)
        headers['x-oss-copy-source'] = "/%s/%s" % (source_bucket, source_object)
        method = 'PUT'
        body = ''
        params = {}
        return self.http_request(method, target_bucket, target_object, headers, body, params)

    def init_multi_upload(self, bucket, object, headers=None, params=None):
        '''
        Init multi upload

        :type bucket: string
        :param

        :type object: string
        :param

        :type headers: dict
        :param: HTTP header

        :type params: dict
        :param: HTTP header

        Returns:
            HTTP Response
        '''
        if not params:
            params = {}
        if not headers:
            headers = {}
        method = 'POST'
        body = ''
        params['uploads'] = ''
        if isinstance(headers, dict) and not headers.has_key('Content-Type'):
            content_type = get_content_type_by_filename(object)
            headers['Content-Type'] = content_type
        return self.http_request(method, bucket, object, headers, body, params)

    def get_all_parts(self, bucket, object, upload_id, max_parts=None, part_number_marker=None, headers=None):
        '''
        List all upload parts of given upload_id
        :type bucket: string
        :param

        :type object: string
        :param

        :type upload_id: string
        :param

        :type max_parts: int
        :param 

        :type part_number_marker: string
        :param

        :type headers: dict
        :param: HTTP header

        Returns:
            HTTP Response
        '''
        method = 'GET'
        if not headers:
            headers = {}
        body = ''
        params = {}
        params['uploadId'] = upload_id
        if max_parts:
            params['max-parts'] = max_parts
        if part_number_marker:
            params['part-number-marker'] = part_number_marker
        return self.http_request(method, bucket, object, headers, body, params)

    def get_all_multipart_uploads(self, bucket, delimiter=None, max_uploads=None, key_marker=None, prefix=None, upload_id_marker=None, headers=None):
        '''
        List all upload_ids and their parts
        :type bucket: string
        :param

        :type delimiter: string
        :param

        :type max_uploads: string
        :param

        :type key_marker: string
        :param

        :type prefix: string
        :param

        :type upload_id_marker: string
        :param

        :type headers: dict
        :param: HTTP header

        Returns:
            HTTP Response
        '''
        method = 'GET'
        object = ''
        body = ''
        params = {}
        params['uploads'] = ''
        if delimiter:
            params['delimiter'] = delimiter
        if max_uploads:
            params['max-uploads'] = max_uploads
        if key_marker:
            params['key-marker'] = key_marker
        if prefix:
            params['prefix'] = prefix
        if upload_id_marker:
            params['upload-id-marker'] = upload_id_marker
        return self.http_request(method, bucket, object, headers, body, params)

    def upload_part(self, bucket, object, filename, upload_id, part_number, headers=None, params=None):
        '''
        Upload the content of filename as one part of given upload_id

        :type bucket: string
        :param

        :type object: string
        :param

        :type filename: string
        :param

        :type upload_id: string
        :param

        :type part_number: int 
        :param

        :type headers: dict
        :param: HTTP header

        :type params: dict
        :param: HTTP header

        Returns:
            HTTP Response
        '''
        if not params:
            params = {}
        params['partNumber'] = part_number
        params['uploadId'] = upload_id
        content_type = ''
        return self.put_object_from_file(bucket, object, filename, content_type, headers, params)

    def upload_part_from_string(self, bucket, object, data, upload_id, part_number, headers=None, params=None):
        '''
        Upload the content of string as one part of given upload_id

        :type bucket: string
        :param

        :type object: string
        :param

        :type data: string
        :param

        :type upload_id: string
        :param

        :type part_number: int 
        :param

        :type headers: dict
        :param: HTTP header

        :type params: dict
        :param: HTTP header

        Returns:
            HTTP Response
        '''
        if not params:
            params = {}
        params['partNumber'] = part_number
        params['uploadId'] = upload_id
        content_type = ''
        fp = StringIO.StringIO(data)
        return self.put_object_from_fp(bucket, object, fp, content_type, headers, params)

    def copy_object_as_part(self, source_bucket, source_object, target_bucket,
                            target_object, upload_id, part_number, headers=None, params=None):
        '''
        Upload a part with data copy from srouce object in source bucket 

        :type source_bucket: string
        :param

        :type source_object: string
        :param

        :type target_bucket: string
        :param

        :type target_object: string
        :param

        :type data: string
        :param

        :type upload_id: string
        :param

        :type part_number: int 
        :param

        :type headers: dict
        :param: HTTP header

        :type params: dict
        :param: HTTP header

        Returns:
            HTTP Response
        '''
        if not headers:
            headers = {}
        if not params:
            params = {}
        source_object = convert_utf8(source_object)
        source_object = oss_quote(source_object)
        method = 'PUT'
        params['partNumber'] = part_number
        params['uploadId'] = upload_id
        headers['x-oss-copy-source'] = "/%s/%s" % (source_bucket, source_object)
        body = ''
        return self.http_request(method, target_bucket, target_object, headers, body, params)

    def complete_upload(self, bucket, object, upload_id, part_msg_xml, headers=None, params=None):
        '''
        Finish multiupload and merge all the parts in part_msg_xml as a object.

        :type bucket: string
        :param

        :type object: string
        :param

        :type upload_id: string
        :param

        :type part_msg_xml: string
        :param

        :type headers: dict
        :param: HTTP header

        :type params: dict
        :param: HTTP header

        Returns:
            HTTP Response
        '''
        if not headers:
            headers = {}
        if not params:
            params = {}
        method = 'POST'
        body = part_msg_xml
        headers['Content-Length'] = str(len(body))
        params['uploadId'] = upload_id
        if not headers.has_key('Content-Type'):
            content_type = get_content_type_by_filename(object)
            headers['Content-Type'] = content_type
        if not headers.has_key('Content-MD5'):
            base64md5 = get_string_base64_md5(body)
            headers['Content-MD5'] = base64md5
        return self.http_request(method, bucket, object, headers, body, params)

    def cancel_upload(self, bucket, object, upload_id, headers=None, params=None):
        '''
        Cancel multiupload and delete all parts of given upload_id
        :type bucket: string
        :param

        :type object: string
        :param

        :type upload_id: string
        :param

        :type headers: dict
        :param: HTTP header

        :type params: dict
        :param: HTTP header

        Returns:
            HTTP Response
        '''
        if not params:
            params = {}
        method = 'DELETE'
        upload_id = convert_utf8(upload_id)
        params['uploadId'] = upload_id
        body = ''
        return self.http_request(method, bucket, object, headers, body, params)

    def multi_upload_file(self, bucket, object, filename, upload_id='', thread_num=10, max_part_num=10000, headers=None, params=None, debug=False, check_md5=False):
        '''
        Upload large file, the content is read from filename. The large file is splitted into many parts. It will        put the many parts into bucket and then merge all the parts into one object.

        :type bucket: string
        :param

        :type object: string
        :param

        :type fllename: string
        :param: the name of the read file

        :type upload_id: string
        :param

        :type thread_num: int
        :param

        :type max_part_num: int
        :param

        :type headers: dict
        :param

        :type params: dict
        :param

        Returns:
            HTTP Response
        '''
        tmp_headers = {}
        if headers and isinstance(headers, dict):
            tmp_headers = headers.copy()
        if not tmp_headers.has_key('Content-Type'):
            content_type = get_content_type_by_filename(filename)
            tmp_headers['Content-Type'] = content_type
        # get init upload_id
        if not upload_id:
            res = self.init_multi_upload(bucket, object, tmp_headers, params)
            body = res.read()
            if res.status == 200:
                h = GetInitUploadIdXml(body)
                upload_id = h.upload_id
            else:
                err = ErrorXml(body)
                raise Exception("%s, %s" % (res.status, err.msg))
        if not upload_id:
            raise Exception("-1, Cannot get upload id.")
        oss = OssAPI(self.host, self.access_id, self.secret_access_key)
        return multi_upload_file2(oss, bucket, object, filename, upload_id, thread_num, max_part_num, self.retry_times, headers, params, debug, check_md5)

    def delete_objects(self, bucket, object_list=None, headers=None, params=None):
        '''
        Batch delete objects
        :type bucket: string
        :param:

        :type object_list: list
        :param:

        :type headers: dict
        :param: HTTP header

        :type params: dict
        :param: the parameters that put in the url address as query string

        Returns:
            HTTP Response
        '''
        if not object_list:
            object_list = []
        object_list_xml = create_delete_object_msg_xml(object_list)
        return self.batch_delete_object(bucket, object_list_xml, headers, params)

    def batch_delete_object(self, bucket, object_list_xml, headers=None, params=None):
        '''
        Delete the objects in object_list_xml
        :type bucket: string
        :param:

        :type object_list_xml: string
        :param:

        :type headers: dict
        :param: HTTP header

        :type params: dict
        :param: the parameters that put in the url address as query string

        Returns:
            HTTP Response
        '''
        if not headers:
            headers = {}
        if not params:
            params = {}
        method = 'POST'
        object = ''
        body = object_list_xml
        headers['Content-Length'] = str(len(body))
        params['delete'] = ''
        base64md5 = get_string_base64_md5(body)
        headers['Content-MD5'] = base64md5
        return self.http_request(method, bucket, object, headers, body, params)

    def list_objects(self, bucket, prefix=''):
        '''
        :type bucket: string
        :param:

        :type prefix: string
        :param:

        Returns:
            a list that contains the objects in bucket with prefix
        '''
        get_instance = GetAllObjects()
        marker_input = ''
        object_list = []
        oss = OssAPI(self.host, self.access_id, self.secret_access_key)
        (object_list, marker_output) = get_instance.get_object_in_bucket(oss, bucket, marker_input, prefix)
        return object_list

    def list_objects_dirs(self, bucket, prefix='', delimiter=''):
        '''
        :type bucket: string
        :param:

        :type prefix: string
        :param:

        :type prefix: delimiter
        :param:

        Returns:
            a list that contains the objects in bucket with prefix
        '''
        get_instance = GetAllObjects()
        marker_input = ''
        object_list = []
        dir_list = []
        oss = OssAPI(self.host, self.access_id, self.secret_access_key)
        (object_list, dir_list) = get_instance.get_all_object_dir_in_bucket(oss, bucket, marker_input, prefix, delimiter)
        return (object_list, dir_list)

    def batch_delete_objects(self, bucket, object_list=None):
        '''
        :type bucket: string
        :param:

        :type object_list: object name list
        :param:

        Returns:
            True or False
        '''
        if not object_list:
            object_list = []
        object_list_xml = create_delete_object_msg_xml(object_list)
        try:
            res = self.batch_delete_object(bucket, object_list_xml)
            if res.status / 100 == 2:
                return True
        except:
            pass
        return False

    def get_object_info(self, bucket, object, headers=None, params=None):
        ''' 
        Get object information
        :type bucket: string
        :param:

        :type object: string
        :param:

        :type headers: dict
        :param: HTTP header

        :type params: dict
        :param: the parameters that put in the url address as query string

        Returns:
            HTTP Response
        '''
        if not headers:
            headers = {}
        if not params:
            params = {}
        method = 'GET'
        body = ''
        params['objectInfo'] = ''
        return self.http_request(method, bucket, object, headers, body, params)

    def options(self, bucket, object='', headers=None, params=None):
        ''' 
        Options object to determine if user can send the actual HTTP request
        :type bucket: string
        :param:

        :type object: string
        :param:

        :type headers: dict
        :param: HTTP header

        :type params: dict
        :param: the parameters that put in the url address as query string

        Returns:
            HTTP Response
        '''
        if not headers:
            headers = {}
        if not params:
            params = {}
        method = 'OPTIONS'
        body = ''
        return self.http_request(method, bucket, object, headers, body, params)

    def put_referer(self, bucket, allow_empty_referer=True, referer_list=None):
        '''
        Put bucket referer

        :type bucket: string
        :param

        :type allow_empty_referer: boolean
        :param

        :type referer_list: list
        :param

        Returns:
            HTTP Response
        '''
        method = 'PUT'
        object = ''
        if allow_empty_referer == True:
            allow = "true"
        elif allow_empty_referer == False:
            allow = "false"
        else:
            allow = "true"
        referer_list_string = ''
        if not referer_list:
            referer_list_string = '<Referer></Referer>'
        elif referer_list and isinstance(referer_list, list):
            for i in referer_list:
                referer_list_string += '<Referer>%s</Referer>' % i.strip()
        else:
            referer_list_string = '<Referer>%s</Referer>' % referer_list
        body = '<RefererConfiguration><AllowEmptyReferer>%s</AllowEmptyReferer><RefererList>%s</RefererList></RefererConfiguration>' % (allow, referer_list_string)
        params = {'referer': ''}
        headers = {}
        headers['Content-Length'] = str(len(body))
        base64md5 = get_string_base64_md5(body)
        headers['Content-MD5'] = base64md5
        return self.http_request(method, bucket, object, headers, body, params)

    def get_referer(self, bucket):
        '''
        Get bucket referer

        :type bucket: string
        :param

        Returns:
            HTTP Response
        '''
        method = 'GET'
        object = ''
        body = ''
        params = {'referer': ''}
        headers = {}
        return self.http_request(method, bucket, object, headers, body, params)
