#!/usr/bin/env python
#coding=utf-8

# Copyright (C) 2011, Alibaba Cloud Computing

#Permission is hereby granted, free of charge, to any person obtaining a copy of this software and associated documentation files (the "Software"), to deal in the Software without restriction, including without limitation the rights to use, copy, modify, merge, publish, distribute, sublicense, and/or sell copies of the Software, and to permit persons to whom the Software is furnished to do so, subject to the following conditions:

#The above copyright notice and this permission notice shall be included in all copies or substantial portions of the Software.

#THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY, FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM, OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE SOFTWARE.

import platform
import urllib
import base64
import hmac
from hashlib import sha1 as sha
import os
import StringIO
from threading import Thread
import Queue
import threading
import ConfigParser
import logging
from logging.handlers import RotatingFileHandler
from xml.sax.saxutils import escape
import socket
import sys
import calendar
import imp
try:
    from oss.oss_xml_handler import *
except:
    from oss_xml_handler import *

def get_md5():
    if sys.version_info >= (2, 6):
        import hashlib
        hash = hashlib.md5()
    else:
        import md5
        hash = md5.new()
    return hash

#LOG_LEVEL can be one of DEBUG INFO ERROR CRITICAL WARNNING
DEBUG = False 
LOG_LEVEL = "DEBUG" 
PROVIDER = "OSS"
SELF_DEFINE_HEADER_PREFIX = "x-oss-"
if "AWS" == PROVIDER:
    SELF_DEFINE_HEADER_PREFIX = "x-amz-"
OSS_HOST_LIST = ["aliyun-inc.com", "aliyuncs.com", "alibaba.net", "s3.amazonaws.com"]
class AtomicInt:
    """
    only the '+=' is atomic
    """
    def __init__(self, v = 0):
        self.lock = threading.Lock()
        self.value = v

    def __add__(self, v):
        return AtomicInt(self.value + v)

    def __radd__(self, v):
        return AtomicInt(self.value + v)

    def __iadd__(self, v):
        self.lock.acquire()
        self.value += v
        self.lock.release()
        return self

    def __repr__(self):
        return str(self.value)

    def __sub__(self, v):
        return AtomicInt(self.value - v)

    def __rsub__(self, v):
        return AtomicInt(v - self.value)

    def __cmp__(self, v):
        if self.value < v:
            return -1
        elif self.value == v:
            return 0
        else: 
            return 1

    def __mod__(self, v):
        return (self.value % v)

class EmptyHandler(logging.Handler):
    def __init__(self):
        self.lock = None
        self.level = None
    def emit(self, record):
        pass
    def handle(self, record):
        pass
    def createLock(self):
        self.lock = None 

class Logger():
    def __init__(self, debug, log_name, log_level, logger):
        self.logger = logging.getLogger(logger)
        if debug:
            logfile = os.path.join(os.getcwd(), log_name)
            max_log_size = 100*1024*1024 #Bytes
            backup_count = 5
            format = \
            "%(asctime)s %(levelname)-8s[%(filename)s:%(lineno)d(%(funcName)s)] %(message)s"
            hdlr = RotatingFileHandler(logfile,
                                          mode='a',
                                          maxBytes=max_log_size,
                                          backupCount=backup_count)
            formatter = logging.Formatter(format)
            hdlr.setFormatter(formatter)
            self.logger.addHandler(hdlr)
            if "DEBUG" == log_level.upper():
                self.logger.setLevel(logging.DEBUG)
            elif "INFO" == log_level.upper():
                self.logger.setLevel(logging.INFO)
            elif "WARNING" == log_level.upper():
                self.logger.setLevel(logging.WARNING)
            elif "ERROR" == log_level.upper():
                self.logger.setLevel(logging.ERROR)
            elif "CRITICAL" == log_level.upper():
                self.logger.setLevel(logging.CRITICAL)
            else:
                self.logger.setLevel(logging.ERROR)
        else:
            self.logger.addHandler(EmptyHandler())

    def getlogger(self):
        return self.logger

OSS_LOGGER_SET = None 
PART_UPLOAD_OK = AtomicInt()
PART_UPLOAD_FAIL = AtomicInt()

def convert_to_localtime(osstimestamp, format="%Y-%m-%dT%H:%M:%S.000Z"):
    ts = format_unixtime(osstimestamp, format)
    return time.strftime("%Y-%m-%d %X", time.localtime(ts))

def format_unixtime(osstimestamp, format="%Y-%m-%dT%H:%M:%S.000Z"):
    imp.acquire_lock()
    try:
        ts = (int)(calendar.timegm(time.strptime(osstimestamp, format)))
    except:
        print "format_unixtime:%s exception, %s, %s" % (osstimestamp, sys.exc_info()[0], sys.exc_info()[1])
    imp.release_lock()
    return ts

def helper_get_host_from_resp(res, bucket):
    host = helper_get_host_from_headers(res.getheaders(), bucket)
    if not host:
        xml = res.read()
        host = RedirectXml(xml).Endpoint().strip()
        host = helper_get_host_from_endpoint(host, bucket)
    return host

def helper_get_host_from_headers(headers, bucket):
    mp = convert_header2map(headers)
    location = safe_get_element('location', mp).strip()
    #https://bucket.oss.aliyuncs.com or http://oss.aliyuncs.com/bucket
    location = location.replace("https://", "").replace("http://", "")
    if location.startswith("%s." % bucket):
        location = location[len(bucket)+1:]
    index = location.find('/')
    if index == -1:
        return location
    return location[:index]

def helper_get_host_from_endpoint(host, bucket):
    index = host.find('/')
    if index != -1:
        host = host[:index]
    index = host.find('\\')
    if index != -1:
        host = host[:index]
    index = host.find(bucket)
    if index == 0:
        host = host[len(bucket)+1:]
    return host

def check_bucket_valid(bucket):
    alphabeta = "abcdefghijklmnopqrstuvwxyz0123456789-"
    if len(bucket) < 3 or len(bucket) > 63:
        return False
    if bucket[-1] == "-" or bucket[-1] == "_":
        return False
    if not ((bucket[0] >= 'a' and bucket[0] <= 'z') or (bucket[0] >= '0' and bucket[0] <= '9')):
        return False
    for i in bucket:
        if not i in alphabeta:
            return False
    return True

def check_redirect(res):
    is_redirect = False
    try:
        if res.status == 301 or res.status == 302:
            is_redirect = True
    except:
        pass
    return is_redirect

########## function for Authorization ##########
def _format_header(headers=None):
    '''
    format the headers that self define
    convert the self define headers to lower.
    '''
    if not headers:
        headers = {}
    tmp_headers = {}
    for k in headers.keys():
        if isinstance(headers[k], unicode):
            headers[k] = convert_utf8(headers[k])

        if k.lower().startswith(SELF_DEFINE_HEADER_PREFIX):
            k_lower = k.lower().strip()
            tmp_headers[k_lower] = headers[k]
        else:
            tmp_headers[k.strip()] = headers[k]
    return tmp_headers

def get_assign(secret_access_key, method, headers=None, resource="/", result=None, debug=DEBUG):
    '''
    Create the authorization for OSS based on header input.
    You should put it into "Authorization" parameter of header.
    '''
    if not headers:
        headers = {}
    if not result:
        result = []
    content_md5 = ""
    content_type = ""
    date = ""
    canonicalized_oss_headers = ""
    secret_access_key = convert_utf8(secret_access_key)
    global OSS_LOGGER_SET 
    if not OSS_LOGGER_SET:
        OSS_LOGGER_SET = Logger(debug, "log.txt", LOG_LEVEL, "oss_util").getlogger()
    OSS_LOGGER_SET.debug("secret_access_key: %s" % secret_access_key)
    content_md5 = safe_get_element('Content-MD5', headers)
    content_type = safe_get_element('Content-Type', headers)
    date = safe_get_element('Date', headers)
    canonicalized_resource = resource
    tmp_headers = _format_header(headers)
    if len(tmp_headers) > 0:
        x_header_list = tmp_headers.keys()
        x_header_list.sort()
        for k in x_header_list:
            if k.startswith(SELF_DEFINE_HEADER_PREFIX):
                canonicalized_oss_headers += "%s:%s\n" % (k, tmp_headers[k]) 
    string_to_sign = method + "\n" + content_md5.strip() + "\n" + content_type + "\n" + date + "\n" + canonicalized_oss_headers + canonicalized_resource
    result.append(string_to_sign)
    OSS_LOGGER_SET.debug("method:%s\n content_md5:%s\n content_type:%s\n data:%s\n canonicalized_oss_headers:%s\n canonicalized_resource:%s\n" % (method, content_md5, content_type, date, canonicalized_oss_headers, canonicalized_resource))
    OSS_LOGGER_SET.debug("string_to_sign:%s\n \nlength of string_to_sign:%d\n" % (string_to_sign, len(string_to_sign)))
    h = hmac.new(secret_access_key, string_to_sign, sha)
    sign_result = base64.encodestring(h.digest()).strip()
    OSS_LOGGER_SET.debug("sign result:%s" % sign_result)
    return sign_result

def get_resource(params=None):
    if not params:
        return ""
    tmp_headers = {}
    for k, v in params.items():
        tmp_k = k.lower().strip()
        tmp_headers[tmp_k] = v
    override_response_list = ['response-content-type', 'response-content-language', 
                              'response-cache-control', 'logging', 'response-content-encoding', 
                              'acl', 'uploadId', 'uploads', 'partNumber', 'group', 'link', 
                              'delete', 'website', 'location', 'objectInfo',
                              'response-expires', 'response-content-disposition', 'cors', 'lifecycle',
                              'restore', 'qos', 'referer', 'append', 'position']
    override_response_list.sort()
    resource = ""
    separator = "?"
    for i in override_response_list:
        if tmp_headers.has_key(i.lower()):
            resource += separator
            resource += i
            tmp_key = str(tmp_headers[i.lower()])
            if len(tmp_key) != 0:
                resource += "="
                resource += tmp_key 
            separator = '&'
    return resource

def oss_quote(in_str):
    if not isinstance(in_str, str):
        in_str = str(in_str)
    return urllib.quote(in_str, '')

def append_param(url, params):
    '''
    convert the parameters to query string of URI.
    '''
    l = []
    for k, v in params.items():
        k = k.replace('_', '-')
        if  k == 'maxkeys':
            k = 'max-keys'
        v = convert_utf8(v) 
        if v is not None and v != '':
            l.append('%s=%s' % (oss_quote(k), oss_quote(v)))
        elif k == 'acl':
            l.append('%s' % (oss_quote(k)))
        elif v is None or v == '':
            l.append('%s' % (oss_quote(k)))
    if len(l):
        url = url + '?' + '&'.join(l)
    return url

############### Construct XML ###############
def create_object_group_msg_xml(part_msg_list=None):
    '''
    get information from part_msg_list and covert it to xml.
    part_msg_list has special format.
    '''
    if not part_msg_list:
        part_msg_list = []
    xml_string = r'<CreateFileGroup>'
    for part in part_msg_list:
        if len(part) >= 3:
            if isinstance(part[1], unicode):
                file_path = convert_utf8(part[1])
            else:
                file_path = part[1]
            file_path = escape(file_path)
            xml_string += r'<Part>'
            xml_string += r'<PartNumber>' + str(part[0]) + r'</PartNumber>'
            xml_string += r'<PartName>' + str(file_path) + r'</PartName>'
            xml_string += r'<ETag>"' + str(part[2]).upper() + r'"</ETag>'
            xml_string += r'</Part>'
        else:
            print "the ", part, " in part_msg_list is not as expected!"
            return ""
    xml_string += r'</CreateFileGroup>'

    return xml_string

def create_object_link_msg_xml_by_name(object_list = None):
    '''
    get information from object_list and covert it to xml.
    '''
    if not object_list:
        object_list = []
    xml_string = r'<CreateObjectLink>'
    for i in range(len(object_list)):
        part = str(object_list[i]).strip()
        file_path = convert_utf8(part)
        file_path = escape(file_path)
        xml_string += r'<Part>'
        xml_string += r'<PartNumber>' + str(i + 1) + r'</PartNumber>'
        xml_string += r'<PartName>' + str(file_path) + r'</PartName>'
        xml_string += r'</Part>'
    xml_string += r'</CreateObjectLink>'

    return xml_string

def create_object_link_msg_xml(part_msg_list = None):
    '''
    get information from part_msg_list and covert it to xml.
    part_msg_list has special format.
    '''
    if not part_msg_list:
        part_msg_list = []
    xml_string = r'<CreateObjectLink>'
    for part in part_msg_list:
        if len(part) >= 2:
            file_path = convert_utf8(part[1])
            file_path = escape(file_path)
            xml_string += r'<Part>'
            xml_string += r'<PartNumber>' + str(part[0]) + r'</PartNumber>'
            xml_string += r'<PartName>' + str(file_path) + r'</PartName>'
            xml_string += r'</Part>'
        else:
            print "the ", part, " in part_msg_list is not as expected!"
            return ""
    xml_string += r'</CreateObjectLink>'

    return xml_string

def create_part_xml(part_msg_list=None):
    '''
    get information from part_msg_list and covert it to xml.
    part_msg_list has special format.
    '''
    if not part_msg_list:
        part_msg_list = []
    xml_string = r'<CompleteMultipartUpload>'
    for part in part_msg_list:
        if len(part) >= 3:
            xml_string += r'<Part>'
            xml_string += r'<PartNumber>' + str(part[0]) + r'</PartNumber>'
            xml_string += r'<ETag>"' + str(part[2]).upper() + r'"</ETag>'
            xml_string += r'</Part>'
        else:
            print "the ", part, " in part_msg_list is not as expected!"
            return ""
    xml_string += r'</CompleteMultipartUpload>'

    return xml_string

def create_delete_object_msg_xml(object_list=None, is_quiet=False, is_defult=False):
    '''
    covert object name list to xml.
    '''
    if not object_list:
        object_list = []
    xml_string = r'<Delete>'
    if not is_defult:
        if is_quiet:
            xml_string += r'<Quiet>true</Quiet>'
        else:
            xml_string += r'<Quiet>false</Quiet>'
    for object in object_list:
        key = convert_utf8(object)
        key = escape(key)
        xml_string += r'<Object><Key>%s</Key></Object>' % key
    xml_string += r'</Delete>'
    return xml_string

############### operate OSS ###############
def clear_all_object_of_bucket(oss_instance, bucket):
    '''
    clean all objects in bucket, after that, it will delete this bucket.
    '''
    return clear_all_objects_in_bucket(oss_instance, bucket)

def clear_all_objects_in_bucket(oss_instance, bucket, delete_marker="", delete_upload_id_marker="", debug=False):
    '''
    it will clean all objects in bucket, after that, it will delete this bucket.

    example:
    from oss_api import *
    host = ""
    id = ""
    key = ""
    oss_instance = OssAPI(host, id, key)
    bucket = "leopublicreadprivatewrite"
    if clear_all_objects_in_bucket(oss_instance, bucket):
        pass
    else:
        print "clean Fail"
    '''
    prefix = ""
    delimiter = ""
    maxkeys = 1000
    try:
        delete_all_objects(oss_instance, bucket, prefix, delimiter, delete_marker, maxkeys, debug)
        delete_all_parts(oss_instance, bucket, delete_marker, delete_upload_id_marker, debug)
        res = oss_instance.delete_bucket(bucket)
        if (res.status / 100 != 2 and res.status != 404):
            print "clear_all_objects_in_bucket: delete bucket:%s fail, ret:%s, request id:%s" % (bucket, res.status, res.getheader("x-oss-request-id"))
            return False
    except socket.error:
        print "socket exception when clear_all_objects_in_bucket:%s from %s" % (bucket, oss_instance.host)
        return False
    return True

def delete_all_objects(oss_instance, bucket, prefix="", delimiter="", delete_marker="", maxkeys=1000, debug=False):
    marker = delete_marker
    delete_obj_num = 0
    oss_encoding_type = 'url'
    while 1:
        object_list = []
        res = oss_instance.get_bucket(bucket, prefix, marker, delimiter, maxkeys, encoding_type=oss_encoding_type)
        if res.status != 200:
            print 'list object in bucket fail'
            print res.status
            print res.read()
            return False
        body = res.read()
        (tmp_object_list, marker) = get_object_list_marker_from_xml(body)
        for item in tmp_object_list:
            object_list.append(urllib.unquote(item[0]))

        if object_list:
            object_list_xml = create_delete_object_msg_xml(object_list)
            res = oss_instance.batch_delete_object(bucket, object_list_xml)
            if res.status/100 != 2:
                if marker:
                    print "delete_all_objects: batch delete objects in bucket:%s fail, ret:%s, request id:%s, first object:%s, marker:%s" % (bucket, res.status, res.getheader("x-oss-request-id"), object_list[0], marker)
                else:
                    print "delete_all_objects: batch delete objects in bucket:%s fail, ret:%s, request id:%s, first object:%s" % (bucket, res.status, res.getheader("x-oss-request-id"), object_list[0])
                return False
            else:
                if debug:
                    delete_obj_num += len(object_list)
                    if marker:
                        print "delete_all_objects: Now %s objects deleted, marker:%s" % (delete_obj_num, marker)
                    else:
                        print "delete_all_objects: Now %s objects deleted" % (delete_obj_num)
        if len(marker) == 0:
            break
        marker = urllib.unquote(marker)
    return True

def delete_all_parts(oss_instance, bucket, delete_object_marker="", delete_upload_id_marker="", debug=False):
    delete_mulitipart_num = 0
    marker = delete_object_marker
    id_marker = delete_upload_id_marker
    while 1:
        res = oss_instance.get_all_multipart_uploads(bucket, key_marker=marker, upload_id_marker=id_marker)
        if res.status != 200:
            break
        body = res.read()
        hh = GetMultipartUploadsXml(body)
        (fl, pl) = hh.list()
        for i in fl:
            object = convert_utf8(i[0])
            res = oss_instance.cancel_upload(bucket, object, i[1])
            if (res.status / 100 != 2 and res.status != 404):
                print "delete_all_parts: cancel upload object:%s, upload_id:%s FAIL, ret:%s, request-id:%s" % (object, i[1], res.status, res.getheader("x-oss-request-id"))
            else:
                delete_mulitipart_num += 1
                if debug:
                    print "delete_all_parts: cancel upload object:%s, upload_id:%s OK\nNow %s parts deleted." % (object, i[1], delete_mulitipart_num)
        if hh.is_truncated:
            marker = hh.next_key_marker
            id_marker = hh.next_upload_id_marker
        else:
            break
        if not marker:
            break

def clean_all_bucket(oss_instance):
    '''
    it will clean all bucket, including the all objects in bucket.
    '''
    res = oss_instance.get_service()
    ret = True
    if (res.status / 100) == 2:
        h = GetServiceXml(res.read())
        for b in h.bucket_list:
            if not clear_all_objects_in_bucket(oss_instance, b.name):
                print "clean bucket ", b.name, " failed! in clean_all_bucket"
                ret = False
        return ret
    else:
        print "failed! get service in clean_all_bucket return ", res.status
        print res.read()
        print res.getheaders()
        return False

def pgfs_clear_all_objects_in_bucket(oss_instance, bucket):
    '''
    it will clean all objects in bucket, after that, it will delete this bucket.
    '''
    b = GetAllObjects()
    b.get_all_object_in_bucket(oss_instance, bucket)
    for i in b.object_list:
        res = oss_instance.delete_object(bucket, i)
        if (res.status / 100 != 2):
            print "clear_all_objects_in_bucket: delete object fail, ret is:", res.status, "bucket is:", bucket, "object is: ", i
            return False
        else:
            pass
    res = oss_instance.delete_bucket(bucket)
    if (res.status / 100 != 2 and res.status != 404):
        print "clear_all_objects_in_bucket: delete bucket fail, ret is: %s, request id is:%s" % (res.status, res.getheader("x-oss-request-id"))
        return False
    return True

def pgfs_clean_all_bucket(oss_instance):
    '''
    it will clean all bucket, including the all objects in bucket.
    '''
    res = oss_instance.get_service()
    if (res.status / 100) == 2:
        h = GetServiceXml(res.read())
        for b in h.bucket_list:
            if not pgfs_clear_all_objects_in_bucket(oss_instance, b.name):
                print "clean bucket ", b.name, " failed! in clean_all_bucket"
                return False
        return True
    else:
        print "failed! get service in clean_all_bucket return ", res.status
        print res.read()
        print res.getheaders()
        return False

def delete_all_parts_of_object_group(oss, bucket, object_group_name):
    res = oss.get_object_group_index(bucket, object_group_name)
    if res.status == 200:
        body = res.read()
        h = GetObjectGroupIndexXml(body)
        object_group_index = h.list()
        for i in object_group_index:
            if len(i) == 4 and len(i[1]) > 0:
                part_name = i[1].strip()
                res = oss.delete_object(bucket, part_name)
                if res.status != 204:
                    print "delete part ", part_name, " in bucket:", bucket, " failed!"
                    return False
    else:
        return False
    return True

def delete_all_parts_of_object_link(oss, bucket, object_link_name):
    res = oss.get_link_index(bucket, object_link_name)
    if res.status == 200:
        body = res.read()
        h = GetObjectLinkIndexXml(body)
        object_link_index = h.list()
        for i in object_link_index:
            if len(i) == 2 and len(i[1]) > 0:
                part_name = i[1].strip()
                res = oss.delete_object(bucket, part_name)
                if res.status != 204:
                    print "delete part ", part_name, " in bucket:", bucket, " failed!"
                    return False
    else:
        return False
    return True

class GetAllObjects:
    def __init__(self):
        self.object_list = []
        self.dir_list = []

    def get_object_in_bucket(self, oss, bucket="", marker="", prefix=""):
        object_list = []
        maxkeys = 1000
        try:
            res = oss.get_bucket(bucket, prefix, marker, maxkeys=maxkeys)
            body = res.read()
            hh = GetBucketXml(body)
            (fl, pl) = hh.list()
            if len(fl) != 0:
                for i in fl:
                    object = convert_utf8(i[0])
                    object_list.append(object)
            marker = hh.nextmarker
        except:
            pass
        return (object_list, marker)
    
    def get_object_dir_in_bucket(self, oss, bucket="", marker="", prefix="", delimiter=""):
        object_list = []
        dir_list = []
        maxkeys = 1000
        try:
            res = oss.get_bucket(bucket, prefix, marker, delimiter, maxkeys=maxkeys)
            body = res.read()
            hh = GetBucketXml(body)
            (fl, pl) = hh.list()
            if len(fl) != 0:
                for i in fl:
                    object_list.append((i[0], i[3], i[1]))  #name, size, modified_time
            if len(pl) != 0:
                for i in pl:
                    dir_list.append(i)
            marker = hh.nextmarker
        except:
            pass
        return (object_list, dir_list, marker)

    def get_all_object_in_bucket(self, oss, bucket="", marker="", prefix=""):
        marker2 = ""
        while True:
            (object_list, marker) = self.get_object_in_bucket(oss, bucket, marker2, prefix)
            marker2 = marker
            if len(object_list) != 0:
                self.object_list.extend(object_list)
            if not marker:
                break

    def get_all_object_dir_in_bucket(self, oss, bucket="", marker="", prefix="", delimiter=""):
        marker2 = ""
        while True:
            (object_list, dir_list, marker) = self.get_object_dir_in_bucket(oss, bucket, marker2, prefix, delimiter)
            marker2 = marker
            if len(object_list) != 0:
                self.object_list.extend(object_list)
            if len(dir_list) != 0:
                self.dir_list.extend(dir_list)
            if not marker:
                break
        return (self.object_list, self.dir_list)

def get_all_buckets(oss):
    bucket_list = []
    res = oss.get_service()
    if res.status == 200:
        h = GetServiceXml(res.read())
        for b in h.bucket_list:
            bucket_list.append(str(b.name).strip())
    return bucket_list 

def get_object_list_marker_from_xml(body):
    #return ([(object_name, object_length, last_modify_time)...], marker)
    object_meta_list = []
    next_marker = ""
    hh = GetBucketXml(body)
    (fl, pl) = hh.list()
    if len(fl) != 0:
        for i in fl:
            object = convert_utf8(i[0])
            last_modify_time = i[1]
            length = i[3]
            etag = i[2]
            object_meta_list.append((object, length, last_modify_time, etag))
    if hh.is_truncated:
        next_marker = hh.nextmarker
    return (object_meta_list, next_marker)

def get_dir_list_marker_from_xml(body):
    #return (dirname, marker)
    dir_list = []
    next_marker = ""
    hh = GetBucketXml(body)
    (fl, pl) = hh.list()
    if len(pl) != 0:
        for i in pl:
            dir_list.append(i)
    if hh.is_truncated:
        next_marker = hh.nextmarker
    return (dir_list, next_marker)

def get_bucket_meta_list_marker_from_xml(body):
    next_marker = ""
    hh = GetServiceXml(body)
    if hh.is_truncated:
        next_marker = hh.nextmarker
    return (hh.bucket_list, next_marker)

def get_upload_id(oss, bucket, object, headers=None):
    '''
    get the upload id of object.
    Returns:
            string
    '''
    if not headers:
        headers = {}
    upload_id = ""
    res = oss.init_multi_upload(bucket, object, headers)
    if res.status == 200:
        body = res.read()
        h = GetInitUploadIdXml(body)
        upload_id = h.upload_id
    else:
        print res.status
        print res.getheaders()
        print res.read()
    return upload_id

def get_all_upload_id_list(oss, bucket, prefix=None):
    '''
    get all upload id of bucket
    Returns:
            list
    '''
    all_upload_id_list = []
    marker = ""
    id_marker = ""
    while True:
        res = oss.get_all_multipart_uploads(bucket, key_marker=marker, prefix=prefix, upload_id_marker=id_marker)
        if res.status != 200:
            return all_upload_id_list

        body = res.read()
        hh = GetMultipartUploadsXml(body)
        (fl, pl) = hh.list()
        for i in fl:
            all_upload_id_list.append(i)
        if hh.is_truncated:
            marker = hh.next_key_marker
            id_marker = hh.next_upload_id_marker
        else:
            break
        if not marker and not id_marker:
            break
    return all_upload_id_list

def get_upload_id_list(oss, bucket, object):
    '''
    get all upload id list of one object.
    Returns:
            list
    '''
    all_upload_id_list = []
    all_upload_id_list = get_all_upload_id_list(oss, bucket, object)
    upload_id_list = []
    if all_upload_id_list:
        for i in all_upload_id_list:
            if "%s" % convert_utf8(object) == "%s" % convert_utf8(i[0]):
                upload_id_list.append(i[1])
    return upload_id_list

def get_part_list(oss, bucket, object, upload_id, max_part=""):
    '''
    get uploaded part list of object.
    Returns:
            list
    '''
    part_list = []
    marker = ""
    while True:
        res = oss.get_all_parts(bucket, object, upload_id, part_number_marker = marker, max_parts=max_part)
        if res.status != 200:
            break
        body = res.read()
        h = GetPartsXml(body)
        part_list.extend(h.list())
        if h.is_truncated:
            marker = h.next_part_number_marker
        else:
            break
        if not marker:
            break
    return part_list

def get_part_xml(oss, bucket, object, upload_id):
    '''
    get uploaded part list of object.
    Returns:
            string
    '''
    part_list = []
    part_list = get_part_list(oss, bucket, object, upload_id)
    xml_string = r'<CompleteMultipartUpload>'
    for part in part_list:
        xml_string += r'<Part>'
        xml_string += r'<PartNumber>' + str(part[0]) + r'</PartNumber>'
        xml_string += r'<ETag>' + part[1] + r'</ETag>'
        xml_string += r'</Part>'
    xml_string += r'</CompleteMultipartUpload>'
    return xml_string

def get_part_map(oss, bucket, object, upload_id):
    part_list = []
    part_list = get_part_list(oss, bucket, object, upload_id)
    part_map = {}
    for part in part_list:
        part_number = str(part[0])
        etag = part[1]
        part_map[part_number] = etag
    return part_map

########## multi-thread ##########
def multi_get(oss, bucket, object, localfile, thread_num, retry_times):
    length = 0
    oss_md5string = ''
    res = oss.head_object(bucket, object)
    if 200 == res.status:
        length = (int)(res.getheader('content-length'))
        oss_md5string = res.getheader('x-oss-meta-md5')
    else:
        print "can not get the length of object:", object
        return False
    ranges = []
    ranges.append(0)
    size = length // thread_num
    for i in xrange(thread_num - 1):
        ranges.append((i + 1) * size)
    ranges.append(length)

    threadpool = []
    for i in xrange(len(ranges) - 1):
        exec("file_%s = open(localfile, 'wb+')" % i)
        exec("current = MultiGetWorker(oss, bucket, object, file_%s, ranges[i], ranges[i + 1] - 1, %s)" % (i, retry_times))
        threadpool.append(current)
        current.start()

    for item in threadpool:
        item.join()
    if oss_md5string != None:
        local_md5string, base64md5 = get_file_md5(localfile)
        if local_md5string != oss_md5string:
            print "localfile:%s md5:%s is not equal with object:%s md5:%s " % (localfile, local_md5string, object, oss_md5string)
            return False
    if not os.path.isfile(localfile) or length != os.path.getsize(localfile):
        print "localfile:%s size:%s is not equal with object:%s size:%s " % (localfile, os.path.getsize(localfile), object, length)
        return False
    else:
        return True

class DeleteObjectWorker(Thread):
    def __init__(self, oss, bucket, part_msg_list, retry_times=5):
        Thread.__init__(self)
        self.oss = oss
        self.bucket = bucket
        self.part_msg_list = part_msg_list
        self.retry_times = retry_times

    def run(self):
        bucket = self.bucket
        object_list = self.part_msg_list
        step = 1000
        begin = 0
        end = 0
        total_length = len(object_list)
        remain_length = total_length
        while True:
            if remain_length > step:
                end = begin + step
            elif remain_length > 0:
                end = begin + remain_length
            else:
                break
            is_fail = True
            retry_times = self.retry_times
            while True:
                try:
                    if retry_times <= 0:
                        break
                    res = self.oss.delete_objects(bucket, object_list[begin:end])
                    if res.status / 100 == 2:
                        is_fail = False
                        break
                except:
                    retry_times = retry_times - 1
                    time.sleep(1)
            if is_fail:
                print "delete object_list[%s:%s] failed!, first is %s" % (begin, end, object_list[begin])
            begin = end
            remain_length = remain_length - step

class PutObjectGroupWorker(Thread):
    def __init__(self, oss, bucket, file_path, part_msg_list, retry_times=5):
        Thread.__init__(self)
        self.oss = oss
        self.bucket = bucket
        self.part_msg_list = part_msg_list
        self.file_path = file_path
        self.retry_times = retry_times

    def run(self):
        for part in self.part_msg_list:
            if len(part) >= 5:
                bucket = self.bucket
                file_name = convert_utf8(part[1])
                object_name = file_name
                retry_times = self.retry_times
                is_skip = False
                while True:
                    try:
                        if retry_times <= 0:
                            break
                        res = self.oss.head_object(bucket, object_name)
                        if res.status == 200:
                            header_map = convert_header2map(res.getheaders())
                            etag = safe_get_element("etag", header_map)
                            md5_str = part[2]
                            if etag.replace('"', "").upper() == md5_str.upper():
                                is_skip = True
                        break
                    except:
                        retry_times = retry_times - 1
                        time.sleep(1)

                if is_skip:
                    continue

                partsize = part[3]
                offset = part[4]
                retry_times = self.retry_times
                while True:
                    try:
                        if retry_times <= 0:
                            break
                        res = self.oss.put_object_from_file_given_pos(bucket, object_name, self.file_path, offset, partsize)
                        if res.status != 200:
                            print "upload ", file_name, "failed!", " ret is:", res.status
                            print "headers", res.getheaders()
                            retry_times = retry_times - 1
                            time.sleep(1)
                        else:
                            break
                    except:
                        retry_times = retry_times - 1
                        time.sleep(1)

            else:
                print "ERROR! part", part , " is not as expected!"

class PutObjectLinkWorker(Thread):
    def __init__(self, oss, bucket, file_path, part_msg_list, retry_times=5):
        Thread.__init__(self)
        self.oss = oss
        self.bucket = bucket
        self.part_msg_list = part_msg_list
        self.file_path = file_path
        self.retry_times = retry_times

    def run(self):
        for part in self.part_msg_list:
            if len(part) >= 5:
                bucket = self.bucket
                file_name = convert_utf8(part[1])
                object_name = file_name
                retry_times = self.retry_times
                is_skip = False
                while True:
                    try:
                        if retry_times <= 0:
                            break
                        res = self.oss.head_object(bucket, object_name)
                        if res.status == 200:
                            header_map = convert_header2map(res.getheaders())
                            etag = safe_get_element("etag", header_map)
                            md5_str = part[2]
                            if etag.replace('"', "").upper() == md5_str.upper():
                                is_skip = True
                        break
                    except:
                        retry_times = retry_times - 1
                        time.sleep(1)

                if is_skip:
                    continue

                partsize = part[3]
                offset = part[4]
                retry_times = self.retry_times
                while True:
                    try:
                        if retry_times <= 0:
                            break
                        res = self.oss.put_object_from_file_given_pos(bucket, object_name, self.file_path, offset, partsize)
                        if res.status != 200:
                            print "upload ", file_name, "failed!", " ret is:", res.status
                            print "headers", res.getheaders()
                            retry_times = retry_times - 1
                            time.sleep(1)
                        else:
                            break
                    except:
                        retry_times = retry_times - 1
                        time.sleep(1)

            else:
                print "ERROR! part", part , " is not as expected!"

def multi_upload_file2(oss, bucket, object, filename, upload_id, thread_num=10, max_part_num=10000, retry_times=5, headers=None, params=None, debug=False, is_check_md5=False):
    if not upload_id:
        print "empty upload_id"
        return False
    part_msg_list = []
    if debug:
        print "split %s to get part list, it may take long time, please wait." % filename
    part_msg_list = split_large_file(filename, object, max_part_num, check_md5=is_check_md5)
    if debug:
        print "split %s finish." % filename
    queue = Queue.Queue(0)
    uploaded_part_map = {}
    part_msg_xml = create_part_xml(part_msg_list)
    each_part_retry_times = 1
    total_parts_num = len(part_msg_list)
    need_upload_parts_num = 0
    for i in range(retry_times):
        tmp_uploaded_part_map = get_part_map(oss, bucket, object, upload_id)
        if tmp_uploaded_part_map:
            for k, v in tmp_uploaded_part_map.items():
                uploaded_part_map[k] = v
        thread_pool = []
        for part in part_msg_list:
            if len(part) >= 5:
                part_number = str(part[0])
                md5_str = part[2]
                is_need_upload = True
                if uploaded_part_map.has_key(part_number):
                    md5_str = part[2]
                    if uploaded_part_map[part_number].replace('"', "").upper() == md5_str.upper():
                        is_need_upload = False
                        continue
                if is_need_upload:
                    queue.put((upload_part, oss, bucket, object, upload_id, filename, part, is_check_md5))
            else:
                print "not expected part", part
        need_upload_parts_num = queue.qsize()
        if debug:
            print "RetryTimes:%s, TotalParts:%s, NeedUploadParts:%s" % (i, total_parts_num, need_upload_parts_num)
        global PART_UPLOAD_OK
        global PART_UPLOAD_FAIL
        PART_UPLOAD_OK = AtomicInt()
        PART_UPLOAD_FAIL = AtomicInt()
                
        for i in xrange(thread_num):
            current = UploadPartWorker2(each_part_retry_times, queue, need_upload_parts_num, debug)
            thread_pool.append(current)
            current.start()
        queue.join()
        for item in thread_pool:
            item.join()

        res = oss.complete_upload(bucket, object, upload_id, part_msg_xml, headers, params)
        if res.status == 200:
            return res
        if res.status > 300 and res.status < 500:
            print res.read()
            raise Exception("-3, bad request, multi upload file failed! upload_id:%s" % (upload_id))
    raise Exception("-3, after retry %s, failed, multi upload file failed! upload_id:%s" % (retry_times, upload_id))
    
def upload_part(oss, bucket, object, upload_id, file_path, part, retry_times=2, is_check_md5=False):
    if len(part) == 6:
        part_number = str(part[0])
        md5_str = part[2]
        partsize = part[3]
        offset = part[4]
        base64md5_str = part[5]
        headers = {}
        if is_check_md5:
            headers["Content-MD5"] = base64md5_str
        for i in range(retry_times):
            try:
                res = oss.upload_part_from_file_given_pos(bucket, object, file_path, offset, partsize, upload_id, part_number, headers)
                if res.status != 200:
                    time.sleep(1)
                else:
                    return True
            except:
                time.sleep(1)
    else:
        print "not expected part for multiupload", part
    return False

class UploadPartWorker2(threading.Thread):
    def __init__(self, retry_times, queue, total_parts_num, debug):
        threading.Thread.__init__(self)
        self.queue = queue
        self.retry_times = retry_times
        self.debug = debug
        self.total_parts_num = total_parts_num

    def run(self):
        global PART_UPLOAD_OK
        global PART_UPLOAD_FAIL
        while 1:
            try:
                (upload_part, oss, bucket, object, upload_id, filename, part, is_check_md5) = self.queue.get(block=False)
                ret = upload_part(oss, bucket, object, upload_id, filename, part, self.retry_times, is_check_md5)
                if ret:
                    PART_UPLOAD_OK += 1
                else:
                    PART_UPLOAD_FAIL += 1
                sum = PART_UPLOAD_OK + PART_UPLOAD_FAIL

                if self.total_parts_num > 0:
                    exec("rate = 100*%s/(%s*1.0)" % (sum, self.total_parts_num))
                else:
                    rate = 0
                if self.debug:
                    print '\rOK:%s, FAIL:%s, TOTAL_DONE:%s, TOTAL_TO_DO:%s, PROCESS:%.2f%%' % (PART_UPLOAD_OK, PART_UPLOAD_FAIL, sum, self.total_parts_num, rate),
                    sys.stdout.flush()
                self.queue.task_done()
            except Queue.Empty:
                break
            except:
                PART_UPLOAD_FAIL += 1
                print sys.exc_info()[0], sys.exc_info()[1]
                self.queue.task_done()

class UploadPartWorker(Thread):
    def __init__(self, oss, bucket, object, upload_id, file_path, part_msg_list, uploaded_part_map, retry_times=5, debug=DEBUG):
        Thread.__init__(self)
        self.oss = oss
        self.bucket = bucket
        self.object = object
        self.part_msg_list = part_msg_list
        self.file_path = file_path
        self.upload_id = upload_id
        self.uploaded_part_map = uploaded_part_map.copy()
        self.retry_times = retry_times

    def run(self):
        for part in self.part_msg_list:
            part_number = str(part[0])
            if len(part) >= 5:
                bucket = self.bucket
                object = self.object
                partsize = part[3]
                offset = part[4]
                retry_times = self.retry_times
                while True:
                    try:
                        if self.uploaded_part_map.has_key(part_number):
                            md5_str = part[2]
                            if self.uploaded_part_map[part_number].replace('"', "").upper() == md5_str.upper():
                                break
                        if retry_times <= 0:
                            break
                        res = self.oss.upload_part_from_file_given_pos(bucket, object, self.file_path, offset, partsize, self.upload_id, part_number)
                        if res.status != 200:
                            retry_times = retry_times - 1
                            time.sleep(1)
                        else:
                            etag = res.getheader("etag")
                            if etag:
                                self.uploaded_part_map[part_number] = etag
                            break
                    except:
                        retry_times = retry_times - 1
                        time.sleep(1)
            else:
                print "not expected part for multiupload", part

class MultiGetWorker(Thread):
    def __init__(self, oss, bucket, object, file, start, end, retry_times=5):
        Thread.__init__(self)
        self.oss = oss
        self.bucket = bucket
        self.object = object
        self.curpos = start
        self.startpos = start
        self.endpos = end
        self.file = file
        self.length = self.endpos - self.startpos + 1
        self.get_buffer_size = 10*1024*1024
        self.retry_times = retry_times

    def run(self):
        if self.startpos > self.endpos:
            return
        retry_times = 0
        totalread = 0
        while True:
            headers = {}
            range_info = 'bytes=%d-%d' % (self.curpos, self.endpos)
            headers['Range'] = range_info
            self.file.seek(self.curpos)
            try:
                res = self.oss.object_operation("GET", self.bucket, self.object, headers)
                if res.status == 206:
                    while True:
                        content = res.read(self.get_buffer_size)
                        if content:
                            self.file.write(content)
                            totalread += len(content)
                            self.curpos += len(content)
                        else:
                            break
                else:
                    print "range get /%s/%s [%s] ret:%s" % (self.bucket, self.object, range_info, res.status)
            except:
                self.file.flush()
                print "range get /%s/%s [%s] exception, retry:%s" % (self.bucket, self.object, range_info, retry_times)
            if totalread == self.length or self.curpos > self.endpos:
                break
            retry_times += 1
            if retry_times > self.retry_times:
                print "ERROR, reach max retry times:%s when multi get /%s/%s" % (self.retry_times, self.bucket, self.object)
                break 
        self.file.flush()
        self.file.close()

############### misc ###############

def split_large_file(file_path, object_prefix="", max_part_num=1000, part_size=10*1024*1024, buffer_size=10*1024*1024, check_md5=False):
    parts_list = []

    if os.path.isfile(file_path):
        file_size = os.path.getsize(file_path)

        if file_size > part_size * max_part_num:
            part_size = (file_size + max_part_num - file_size % max_part_num) / max_part_num

        part_order = 1
        fp = open(file_path, 'rb')
        fp.seek(os.SEEK_SET)

        part_num = (file_size + part_size - 1) / part_size

        for i in xrange(0, part_num):
            left_len = part_size
            real_part_size = 0
            base64md5 = None
            m = get_md5()
            offset = part_size * i
            while True:
                read_size = 0
                if left_len <= 0:
                    break
                elif left_len < buffer_size:
                    read_size = left_len
                else:
                    read_size = buffer_size

                buffer_content = fp.read(read_size)
                m.update(buffer_content)
                real_part_size += len(buffer_content)

                left_len = left_len - read_size

            md5sum = m.hexdigest()
            if check_md5:
                base64md5 = base64.encodestring(m.digest()).strip()

            temp_file_name = os.path.basename(file_path) + "_" + str(part_order)
            object_prefix = convert_utf8(object_prefix)
            if not object_prefix:
                file_name = sum_string(temp_file_name) + "_" + temp_file_name
            else:
                file_name = object_prefix + "/" + sum_string(temp_file_name) + "_" + temp_file_name
            part_msg = (part_order, file_name, md5sum, real_part_size, offset, base64md5)
            parts_list.append(part_msg)
            part_order += 1

        fp.close()
    else:
        print "ERROR! No file: ", file_path, ", please check."

    return parts_list

BUFFER_SIZE = 10*1024*1024
def sumfile(fobj):
    '''Returns an md5 hash for an object with read() method.'''
    m = get_md5()
    while True:
        d = fobj.read(BUFFER_SIZE)
        if not d:
            break
        m.update(d)
    return m.hexdigest()

def md5sum(fname):
    '''Returns an md5 hash for file fname, or stdin if fname is "-".'''
    if fname == '-':
        ret = sumfile(sys.stdin)
    else:
        try:
            f = file(fname, 'rb')
            ret = sumfile(f)
            f.close()
        except:
            return 'Failed to get file:%s md5' % fname
    return ret

def md5sum2(filename, offset=0, partsize=0):
    m = get_md5()
    fp = open(filename, 'rb')
    if offset > os.path.getsize(filename):
        fp.seek(os.SEEK_SET, os.SEEK_END)
    else:
        fp.seek(offset)

    left_len = partsize
    BufferSize = BUFFER_SIZE
    while True:
        if left_len <= 0:
            break
        elif left_len < BufferSize:
            buffer_content = fp.read(left_len)
        else:
            buffer_content = fp.read(BufferSize)
        m.update(buffer_content)
        left_len = left_len - len(buffer_content)
    md5sum = m.hexdigest()
    return md5sum

def sum_string(content):
    f = StringIO.StringIO(content)
    md5sum = sumfile(f)
    f.close()
    return md5sum

def convert_header2map(header_list):
    header_map = {}
    for (a, b) in header_list:
        header_map[a] = b
    return header_map

def safe_get_element(name, container):
    for k, v in container.items():
        if k.strip().lower() == name.strip().lower():
            return v
    return ""

def get_content_type_by_filename(file_name):
    mime_type = ""
    mime_map = {}
    mime_map["js"] = "application/javascript"
    mime_map["xlsx"] = "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet"
    mime_map["xltx"] = "application/vnd.openxmlformats-officedocument.spreadsheetml.template"
    mime_map["potx"] = "application/vnd.openxmlformats-officedocument.presentationml.template"
    mime_map["ppsx"] = "application/vnd.openxmlformats-officedocument.presentationml.slideshow"
    mime_map["pptx"] = "application/vnd.openxmlformats-officedocument.presentationml.presentation"
    mime_map["sldx"] = "application/vnd.openxmlformats-officedocument.presentationml.slide"
    mime_map["docx"] = "application/vnd.openxmlformats-officedocument.wordprocessingml.document"
    mime_map["dotx"] = "application/vnd.openxmlformats-officedocument.wordprocessingml.template"
    mime_map["xlam"] = "application/vnd.ms-excel.addin.macroEnabled.12"
    mime_map["xlsb"] = "application/vnd.ms-excel.sheet.binary.macroEnabled.12"
    mime_map["apk"] = "application/vnd.android.package-archive"
    try:
        suffix = ""
        name = os.path.basename(file_name)
        suffix = name.split('.')[-1]
        if suffix in mime_map.keys():
            mime_type = mime_map[suffix] 
        else:
            import mimetypes
            mimetypes.init()
            mime_type = mimetypes.types_map["." + suffix]
    except Exception:
        mime_type = 'application/octet-stream'
    if not mime_type:
        mime_type = 'application/octet-stream'
    return mime_type

def smart_code(input_stream):
    if isinstance(input_stream, str):
        try:
            tmp = unicode(input_stream, 'utf-8')
        except UnicodeDecodeError:
            try:
                tmp = unicode(input_stream, 'gbk')
            except UnicodeDecodeError:
                try:
                    tmp = unicode(input_stream, 'big5')
                except UnicodeDecodeError:
                    try:
                        tmp = unicode(input_stream, 'ascii')
                    except:
                        tmp = input_stream
    else:
        tmp = input_stream
    return tmp

def is_ip(s):
    try:
        tmp_list = s.split(':')
        s = tmp_list[0]
        if s == 'localhost':
            return True
        tmp_list = s.split('.')
        if len(tmp_list) != 4:
            return False
        else:
            for i in tmp_list:
                if int(i) < 0 or int(i) > 255:
                    return False
    except:
        return False
    return True

def get_host_from_list(hosts):
    tmp_list = hosts.split(",")
    if len(tmp_list) <= 1:
        return hosts
    for tmp_host in tmp_list:
        tmp_host = tmp_host.strip()
        host = tmp_host
        port = 80
        try:
            host_port_list = tmp_host.split(":")
            if len(host_port_list) == 1:
                host = host_port_list[0].strip()
            elif len(host_port_list) == 2:
                host = host_port_list[0].strip()
                port = int(host_port_list[1].strip())
            sock=socket.socket(socket.AF_INET,socket.SOCK_STREAM)
            sock.connect((host, port))
            return host
        except:
            pass
    return tmp_list[0].strip()
    
def is_oss_host(host, is_oss_host=False):
    if is_oss_host:
        return True
    for i in OSS_HOST_LIST: 
        if host.find(i) != -1:
            return True
    return False

def convert_utf8(input_string):
    if isinstance(input_string, unicode):
        input_string = input_string.encode('utf-8')
    return input_string

def get_string_base64_md5(string):
    fd = StringIO.StringIO(string)
    base64md5 = get_fp_base64_md5(fd)
    fd.close()
    return base64md5
    
def get_file_base64_md5(file):
    fd = open(file, 'rb')
    base64md5 = get_fp_base64_md5(fd)
    fd.close()
    return base64md5

def get_fp_base64_md5(fd):
    m = get_md5()
    while True:
        d = fd.read(BUFFER_SIZE)
        if not d:
            break
        m.update(d)
    base64md5 = base64.encodestring(m.digest()).strip()
    return base64md5

def get_file_md5(file):
    fd = open(file, 'rb')
    md5string, base64md5 = get_fp_md5(fd)
    fd.close()
    return md5string, base64md5

def get_fp_md5(fd):
    m = get_md5()
    while True:
        d = fd.read(BUFFER_SIZE)
        if not d:
            break
        m.update(d)
    md5string = m.hexdigest()
    base64md5 = base64.encodestring(m.digest()).strip()
    return md5string, base64md5

def get_unique_temp_filename(temp_dir, localfile):
    import random, string
    def random_str(randomlength=8):
        a = list(string.ascii_letters)
        random.shuffle(a)
        return ''.join(a[:randomlength])

    while True:
        suffix = random_str(8)
        filename_let = [temp_dir, '/.', os.path.basename(localfile), '.', suffix]
        temp_filename = ''.join(filename_let)
        if not os.path.exists(temp_filename):
            return temp_filename

if __name__ == '__main__':
    pass
