# -*- coding: utf-8 -*-
'''
Created on 2012-7-3

@author: lijie.ma
'''

try: import httplib
except ImportError:
    import http.client as httplib
import sys
import urllib
import time
import json
import aliyun
import itertools
import mimetypes
import base64
import hmac
import uuid
from hashlib import sha1

def sign(accessKeySecret, parameters):
    #===========================================================================
    # '''签名方法
    # @param secret: 签名需要的密钥
    # @param parameters: 支持字典和string两种
    # '''
    #===========================================================================
    # 如果parameters 是字典类的话
    sortedParameters = sorted(parameters.items(), key=lambda parameters: parameters[0])

    canonicalizedQueryString = ''
    for (k,v) in sortedParameters:
        canonicalizedQueryString += '&' + percent_encode(k) + '=' + percent_encode(v)

    stringToSign = 'POST&%2F&' + percent_encode(canonicalizedQueryString[1:])

    h = hmac.new(accessKeySecret + "&", stringToSign, sha1)
    signature = base64.encodestring(h.digest()).strip()
    return signature

def percent_encode(encodeStr):
    encodeStr = str(encodeStr)
    res = urllib.quote(encodeStr.decode(sys.stdin.encoding).encode('utf8'), '')
    res = res.replace('+', '%20')
    res = res.replace('*', '%2A')
    res = res.replace('%7E', '~')
    return res

def mixStr(pstr):
    if(isinstance(pstr, str)):
        return pstr
    elif(isinstance(pstr, unicode)):
        return pstr.encode('utf-8')
    else:
        return str(pstr)
    
class FileItem(object):
    def __init__(self,filename=None,content=None):
        self.filename = filename
        self.content = content

class MultiPartForm(object):
    """Accumulate the data to be used when posting a form."""

    def __init__(self):
        self.form_fields = []
        self.files = []
        self.boundary = "PYTHON_SDK_BOUNDARY"
        return
    
    def get_content_type(self):
        return 'multipart/form-data; boundary=%s' % self.boundary

    def add_field(self, name, value):
        """Add a simple field to the form data."""
        self.form_fields.append((name, str(value)))
        return

    def add_file(self, fieldname, filename, fileHandle, mimetype=None):
        """Add a file to be uploaded."""
        body = fileHandle.read()
        if mimetype is None:
            mimetype = mimetypes.guess_type(filename)[0] or 'application/octet-stream'
        self.files.append((mixStr(fieldname), mixStr(filename), mixStr(mimetype), mixStr(body)))
        return
    
    def __str__(self):
        """Return a string representing the form data, including attached files."""
        # Build a list of lists, each containing "lines" of the
        # request.  Each part is separated by a boundary string.
        # Once the list is built, return a string where each
        # line is separated by '\r\n'.  
        parts = []
        part_boundary = '--' + self.boundary
        
        # Add the form fields
        parts.extend(
            [ part_boundary,
              'Content-Disposition: form-data; name="%s"' % name,
              'Content-Type: text/plain; charset=UTF-8',
              '',
              value,
            ]
            for name, value in self.form_fields
            )
        
        # Add the files to upload
        parts.extend(
            [ part_boundary,
              'Content-Disposition: file; name="%s"; filename="%s"' % \
                 (field_name, filename),
              'Content-Type: %s' % content_type,
              'Content-Transfer-Encoding: binary',
              '',
              body,
            ]
            for field_name, filename, content_type, body in self.files
            )
        
        # Flatten the list and add closing boundary marker,
        # then return CR+LF separated data
        flattened = list(itertools.chain(*parts))
        flattened.append('--' + self.boundary + '--')
        flattened.append('')
        return '\r\n'.join(flattened)

class AliyunException(Exception):
    #===========================================================================
    # 业务异常类
    #===========================================================================
    def __init__(self):
        self.code = None
        self.message = None
        self.host = None
        self.requestId = None
    
    def __str__(self, *args, **kwargs):
        sb = "code=" + mixStr(self.code) +\
            " message=" + mixStr(self.message) +\
            " host=" + mixStr(self.host) +\
            " requestId=" + mixStr(self.requestId)
        return sb
       
class RequestException(Exception):
    #===========================================================================
    # 请求连接异常类
    #===========================================================================
    pass

class RestApi(object):
    #===========================================================================
    # Rest api的基类
    #===========================================================================
    
    def __init__(self, domain, port = 80):
        #=======================================================================
        # 初始化基类
        # Args @param domain: 请求的域名或者ip
        #      @param port: 请求的端口
        #=======================================================================
        self.__domain = domain
        self.__port = port
        self.__httpmethod = "POST"
        if(aliyun.getDefaultAppInfo()):
            self.__access_key_id = aliyun.getDefaultAppInfo().accessKeyId
            self.__access_key_secret = aliyun.getDefaultAppInfo().accessKeySecret
        
    def get_request_header(self):
        return {
                 'Content-type': 'application/x-www-form-urlencoded',
                 "Cache-Control": "no-cache",
                 "Connection": "Keep-Alive",
        }
        
    def set_app_info(self, appinfo):
        #=======================================================================
        # 设置请求的app信息
        # @param appinfo: import aliyun
        #                 appinfo aliyun.appinfo(accessKeyId,accessKeySecret)
        #=======================================================================
        self.__access_key_id = appinfo.accessKeyId
        self.__access_key_secret = appinfo.accessKeySecret
        
    def getapiname(self):
        return ""
    
    def getMultipartParas(self):
        return [];

    def getTranslateParas(self):
        return {};
    
    def _check_requst(self):
        pass
    
    def getResponse(self, authrize=None, timeout=30):
        #=======================================================================
        # 获取response结果
        #=======================================================================
        connection = httplib.HTTPConnection(self.__domain, self.__port, timeout)
        timestamp = time.strftime("%Y-%m-%dT%H:%M:%SZ", time.gmtime())
        apiname_split = self.getapiname().split(".")
        parameters = { \
                'Format'        : 'json', \
                'Version'   : apiname_split[4], \
                'Action'        : apiname_split[3], \
                'AccessKeyId'   : self.__access_key_id, \
                'SignatureVersion'  : '1.0', \
                'SignatureMethod'   : 'HMAC-SHA1', \
                'SignatureNonce'    : str(uuid.uuid1()), \
                'TimeStamp'         : timestamp, \
                'partner_id'        : '1.0',\
        }
        application_parameter = self.getApplicationParameters()
        for key in application_parameter.keys():
            parameters[key] = application_parameter[key]

        signature = sign(self.__access_key_secret,parameters)
        parameters['Signature'] = signature
        url = "/?" + urllib.urlencode(parameters)
        
        connection.connect()
        
        header = self.get_request_header();
        if(self.getMultipartParas()):
            form = MultiPartForm()
            for key in self.getMultipartParas():
                fileitem = getattr(self,key)
                if(fileitem and isinstance(fileitem,FileItem)):
                    form.add_file(key,fileitem.filename,fileitem.content)
            body = str(form)
            header['Content-type'] = form.get_content_type()
        else:
            body = None   
        connection.request(self.__httpmethod, url, body=body, headers=header)
        response = connection.getresponse();
        result = response.read()
        jsonobj = json.loads(result)
        return jsonobj
    
    
    def getApplicationParameters(self):
        application_parameter = {}
        for key, value in self.__dict__.iteritems():
            if not key.startswith("__") and not key in self.getMultipartParas() and not key.startswith("_RestApi__") and value is not None :
                if(key.startswith("_")):
                    application_parameter[key[1:]] = value
                else:
                    application_parameter[key] = value
        #查询翻译字典来规避一些关键字属性
        translate_parameter = self.getTranslateParas()
        for key, value in application_parameter.iteritems():
            if key in translate_parameter:
                application_parameter[translate_parameter[key]] = application_parameter[key]
                del application_parameter[key]
        return application_parameter
