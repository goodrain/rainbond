#!/usr/bin/env python
#coding=utf8
import os
import time
import sys
import optparse
from optparse import OptionParser
from oss_api import *
from oss_xml_handler import *

DEBUG = False 
INTERVAL = 1
def check_res(res, msg, cost):
    request_id = res.getheader('x-oss-request-id')
    if (res.status / 100) == 2:
        if DEBUG:
            print "request_id:%s, %s OK, cost:%s ms" % (request_id, msg, cost)
            return 0
    else:
        print "request_id:%s, %s FAIL, cost:%s ms, ret:%s" % (request_id, msg, cost, res.status)
        return 1

def check(res, msg, begin_time):
    end_time = time.time()
    cost = "%.2f" % (1000*(end_time - begin_time))
    time.sleep(INTERVAL)
    return check_res(res, msg, cost)

if __name__ == "__main__": 
    parser = OptionParser()
    parser.add_option("-H", "--host", dest="host", help="specify ")
    parser.add_option("", "--id", dest="id", help="specify ")
    parser.add_option("", "--key", dest="key", help="specify ")
    parser.add_option("", "--bucket", dest="bucket", help="specify ")
    parser.add_option("", "--debug", dest="debug", help="specify ")
    parser.add_option("", "--interval", dest="interval", help="specify ")
    (opts, args) = parser.parse_args()
    if opts.debug is not None:
        DEBUG = True
    HOST = opts.host
    ID = opts.id
    KEY = opts.key
    BUCKET = opts.bucket
    if opts.interval:
        INTERVAL = (float)(opts.interval)
    if not HOST or not ID or not KEY or not BUCKET:
        print "python %s --host=xxx --id=xxx --key=xxx --bucket=xxx" % __file__
        print "For example: \npython %s --host=oss-cn-hangzhou.aliyuncs.com --id=your-id --key=your-key --bucket=testbucket%s" % (__file__, (int)(time.time()))
        exit(-1)
    for host in [HOST]:
        is_test_ok = False
        oss = OssAPI(host, ID, KEY)
        start_time = time.time()
        bucket = BUCKET 
        
        #bucket 相关接口
        #创建bucket
        b_ = time.time()
        acl = 'private'
        headers = {}
        res = oss.put_bucket(bucket, acl, headers)
        msg = "create bucket:%s" % (bucket)
        if check(res, msg, b_):
            break

        #获取bucket的权限
        b_ = time.time()
        res = oss.get_bucket_acl(bucket)
        msg = "get bucket acl:%s" % (bucket)
        if check(res, msg, b_):
            break

        #查看bucket所属的数据中心
        b_ = time.time()
        acl = 'private'
        headers = {}
        res = oss.get_bucket_location(bucket)
        msg = "get bucket location:%s" % (bucket)
        if check(res, msg, b_):
            break

        #设置bucket的CORS
        b_ = time.time()
        cors_xml = '''<CORSConfiguration>
                         <CORSRule>
                           <AllowedOrigin>http://www.example.com</AllowedOrigin>
                           <AllowedMethod>PUT</AllowedMethod>
                           <AllowedMethod>POST</AllowedMethod>
                           <AllowedMethod>DELETE</AllowedMethod>
                           <AllowedHeader>*</AllowedHeader>
                           <MaxAgeSeconds>3000</MaxAgeSeconds>
                           <ExposeHeader>test</ExposeHeader>
                           <ExposeHeader>test2</ExposeHeader>
                         </CORSRule>
                         <CORSRule>
                           <AllowedOrigin>*</AllowedOrigin>
                           <AllowedMethod>GET</AllowedMethod>
                         </CORSRule>
                      </CORSConfiguration>'''
        res = oss.put_cors(bucket, cors_xml)
        msg = "put cors:%s" % (bucket)
        if check(res, msg, b_):
            break

        #读取bucket的CORS
        b_ = time.time()
        res = oss.get_cors(bucket)
        msg = "get cors:%s" % (bucket)
        if check(res, msg, b_):
            break
        
        #删除bucket的CORS
        b_ = time.time()
        res = oss.delete_cors(bucket)
        msg = "delete cors:%s" % (bucket)
        if check(res, msg, b_):
            break

        #设置bucket的website
        b_ = time.time()
        index_file = "index.html"
        error_file = "404.html"
        res = oss.put_website(bucket, index_file, error_file)
        msg = "put website:%s" % (bucket)
        if check(res, msg, b_):
            break

        #读取bucket的website
        b_ = time.time()
        res = oss.get_website(bucket)
        msg = "get website:%s" % (bucket)
        if check(res, msg, b_):
            break

        #删除bucket的website
        b_ = time.time()
        res = oss.delete_website(bucket)
        msg = "delete website:%s" % (bucket)
        if check(res, msg, b_):
            break

        #设置bucket的lifecycle
        b_ = time.time()
        lifecycle = '''
            <LifecycleConfiguration>
              <Rule>
                <ID>1125</ID>
                <Prefix>12</Prefix>
                <Status>Enabled</Status>
                <Expiration>
                  <Days>2</Days>
                </Expiration>
              </Rule>
            </LifecycleConfiguration>'''
        res = oss.put_lifecycle(bucket, lifecycle)
        msg = "put lifecycle:%s" % (bucket)
        if check(res, msg, b_):
            break

        #读取bucket的lifecycle
        b_ = time.time()
        res = oss.get_lifecycle(bucket)
        msg = "get lifecycle:%s" % (bucket)
        if check(res, msg, b_):
            break
        
        #删除bucket的lifecycle
        b_ = time.time()
        res = oss.delete_lifecycle(bucket)
        msg = "delete lifecycle:%s" % (bucket)
        if check(res, msg, b_):
            break

        #设置bucket的logging
        b_ = time.time()
        prefix = "logging-prefix"
        res = oss.put_logging(bucket, bucket, prefix)
        msg = "put bucket logging:%s" % (bucket)
        if check(res, msg, b_):
            break
        
        #读取bucket的logging
        b_ = time.time()
        res = oss.get_logging(bucket)
        msg = "get bucket logging:%s" % (bucket)
        if check(res, msg, b_):
            break
        
        #删除bucket的logging
        b_ = time.time()
        res = oss.delete_logging(bucket)
        msg = "delete bucket logging:%s" % (bucket)
        if check(res, msg, b_):
            break

        #列出创建的bucket
        b_ = time.time()
        res = oss.get_service()
        msg = "get service"
        if check(res, msg, b_):
            break
        
        #object相关：
        #把指定的字符串内容上传到bucket中,在bucket中的文件名叫object。
        b_ = time.time()
        object = "object_test"
        input_content = "hello, OSS"
        content_type = "text/HTML"
        headers = {}
        res = oss.put_object_from_string(bucket, object, input_content, content_type, headers)
        msg = "put /%s/%s from string" % (bucket, object)
        if check(res, msg, b_):
            break 

        #调用copy接口
        b_ = time.time()
        headers = {}
        headers["x-oss-meta-test"] = "oss-test-meta"
        res = oss.copy_object(bucket, object, bucket, object, headers)
        msg = "copy /%s/%s " % (bucket, object)
        if check(res, msg, b_):
            break 
        
        #指定文件名, 把这个文件上传到bucket中,在bucket中的文件名叫object。
        b_ = time.time()
        filename = __file__ 
        content_type = "text/HTML"
        headers = {}
        res = oss.put_object_from_file(bucket, object, filename, content_type, headers)
        msg = "put /%s/%s from file" % (bucket, object)
        if check(res, msg, b_):
            break 
     
        #下载bucket中的object，内容在body中
        b_ = time.time()
        headers = {}
        res = oss.get_object(bucket, object, headers)
        msg = "get /%s/%s" % (bucket, object)
        if check(res, msg, b_):
            break

        #下载bucket中的object，把内容写入到本地文件中
        b_ = time.time()
        headers = {}
        filename = "get_object_test_file"
        res = oss.get_object_to_file(bucket, object, filename, headers)
        msg = "get /%s/%s to %s" % (bucket, object, filename)
        if filename:
            os.remove(filename)
        if check(res, msg, b_):
            break

        #查看object的meta 信息，例如长度，类型等
        b_ = time.time()
        headers = {}
        res = oss.head_object(bucket, object, headers)
        msg = "head /%s/%s" % (bucket, object)
        if check(res, msg, b_):
            break
        
        #列出bucket中所拥有的object
        b_ = time.time()
        prefix = ""
        marker = ""
        delimiter = "/"
        maxkeys = "100"
        headers = {}
        res = oss.get_bucket(bucket, prefix, marker, delimiter, maxkeys, headers)
        msg = "list bucket %s" % bucket
        if check(res, msg, b_):
            break
     
        #multipart相关
        #初始化一个upload_id
        b_ = time.time()
        res = oss.init_multi_upload(bucket, object)
        msg = "init multipart %s" % bucket
        if check(res, msg, b_):
            break
        body = res.read()
        h = GetInitUploadIdXml(body)
        upload_id = h.upload_id
        
        #删除upload_id相关的part
        b_ = time.time()
        res = oss.cancel_upload(bucket, object, upload_id)
        msg = "cancel multpart %s" % bucket
        if check(res, msg, b_):
            break

        #再次初始化
        b_ = time.time()
        res = oss.init_multi_upload(bucket, object)
        msg = "init multipart %s" % bucket
        if check(res, msg, b_):
            break
        body = res.read()
        h = GetInitUploadIdXml(body)
        upload_id = h.upload_id

        #上传part
        b_ = time.time()
        part_number = 1
        res = oss.upload_part(bucket, object, __file__, upload_id, part_number)
        msg = "upload part /%s/%s" % (bucket, object)
        if check(res, msg, b_):
            break
        
        #通过upload_id查看上传了多少块
        b_ = time.time()
        res = oss.get_all_parts(bucket, object, upload_id)
        msg = "get parts of /%s/%s by upload_id" % (bucket, object)
        if check(res, msg, b_):
            break

        #查看bucket中有多少正在上传的multipart
        b_ = time.time()
        res = oss.get_all_multipart_uploads(bucket, delimiter=None, max_uploads=None, key_marker=None, prefix=None, upload_id_marker=None)
        msg = "get all uploads in %s" % bucket
        if check(res, msg, b_):
            break

        #完成multipart上传
        b_ = time.time()
        part_msg_xml = get_part_xml(oss, bucket, object, upload_id)
        res = oss.complete_upload(bucket, object, upload_id, part_msg_xml)
        msg = "complete multipart /%s/%s" % (bucket, object)
        if check(res, msg, b_):
            break

        #删除bucket中的object
        b_ = time.time()
        headers = {}
        if (int)(b_) % 2 == 0:
            res = oss.delete_object(bucket, object, headers)
            msg = "delete object /%s/%s" % (bucket, object)
            if check(res, msg, b_):
                break
        else:
            res = oss.delete_objects(bucket, [object])
            msg = "delete objects in /%s" % (bucket)
            if check(res, msg, b_):
                break

        end_time = time.time()
        if DEBUG:
            print "check %s costs:%s s" % (host, (end_time - start_time)) 
        is_test_ok = True
    if is_test_ok:
        print "%s test OK" % host
    else:
        print "%s test FAIL" % host
