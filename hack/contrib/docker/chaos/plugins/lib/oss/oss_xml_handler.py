#!/usr/bin/env python
#coding=utf-8

# Copyright (C) 2011, Alibaba Cloud Computing

#Permission is hereby granted, free of charge, to any person obtaining a copy of this software and associated documentation files (the "Software"), to deal in the Software without restriction, including without limitation the rights to use, copy, modify, merge, publish, distribute, sublicense, and/or sell copies of the Software, and to permit persons to whom the Software is furnished to do so, subject to the following conditions:

#The above copyright notice and this permission notice shall be included in all copies or substantial portions of the Software.

#THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY, FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM, OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE SOFTWARE.

from xml.dom import minidom
import time
import sys
from xml.sax.saxutils import escape, unescape

XML_UNESCAPE_TABLE = {
    "&#26;" : ''
}

def get_md5():
    if sys.version_info >= (2, 6):
        import hashlib
        hash = hashlib.md5()
    else:
        import md5
        hash = md5.new()
    return hash

def get_xml_unescape_table():
    return XML_UNESCAPE_TABLE
    
def get_xml_unescape_map():
    xml_map = {}
    for k, v in XML_UNESCAPE_TABLE.items():
        m = get_md5()
        m.update(k)
        md5_k = m.hexdigest()
        xml_map[k] = md5_k + str(time.time())
    return xml_map

def has_tag(element, tag):
    nodes = element.getElementsByTagName(tag)
    if len(nodes):
        return True
    else:
        return False

def get_tag_text(element, tag, convert_to_bool = True):
    nodes = element.getElementsByTagName(tag)
    if len(nodes) == 0:
        return ""
    else:
        node = nodes[0]
    rc = ""
    for node in node.childNodes:
        if node.nodeType in ( node.TEXT_NODE, node.CDATA_SECTION_NODE):
            rc = rc + node.data
    if convert_to_bool:
        if rc == "true":
            return True
        elif rc == "false":
            return False
    return rc

class ErrorXml:
    def __init__(self, xml_string):
        self.xml = minidom.parseString(xml_string)
        self.code = get_tag_text(self.xml, 'Code')
        self.msg = get_tag_text(self.xml, 'Message')
        self.resource = get_tag_text(self.xml, 'Resource')
        self.request_id = get_tag_text(self.xml, 'RequestId')
        self.host_id = get_tag_text(self.xml, 'HostId')
    
    def show(self):
        print "Code: %s\nMessage: %s\nResource: %s\nRequestId: %s \nHostId: %s" % (self.code, self.msg, self.resource, self.request_id, self.host_id)

class Owner:
    def __init__(self, xml_element):
        self.element = xml_element
        self.id = get_tag_text(self.element, "ID")
        self.display_name = get_tag_text(self.element, "DisplayName")
    
    def show(self):
        print "ID: %s\nDisplayName: %s" % (self.id, self.display_name)

class Bucket:
    def __init__(self, xml_element):
        self.element = xml_element
        self.location = get_tag_text(self.element, "Location")
        self.name = get_tag_text(self.element, "Name", convert_to_bool = False)
        self.creation_date = get_tag_text(self.element, "CreationDate")
    
    def show(self):
        print "Name: %s\nCreationDate: %s\nLocation: %s" % (self.name, self.creation_date, self.location)

class GetServiceXml:
    def __init__(self, xml_string):
        self.xml = minidom.parseString(xml_string)
        self.owner = Owner(self.xml.getElementsByTagName('Owner')[0])
        self.buckets = self.xml.getElementsByTagName('Bucket')
        self.bucket_list = []
        self.prefix = get_tag_text(self.xml, 'Prefix', convert_to_bool = False)
        self.marker = get_tag_text(self.xml, 'Marker', convert_to_bool = False)
        self.maxkeys = get_tag_text(self.xml, 'MaxKeys')
        self.is_truncated = get_tag_text(self.xml, 'IsTruncated')
        self.nextmarker = get_tag_text(self.xml, 'NextMarker')

        for b in self.buckets:
            self.bucket_list.append(Bucket(b))

    def show(self):
        print "Owner:"
        self.owner.show()
        print "\nBucket list:"
        for b in self.bucket_list:
            b.show()
            print ""

    def list(self):
        bl = []
        for b in self.bucket_list:
            bl.append((b.name, b.creation_date, b.location))
        return bl
    
    def get_prefix(self):
        return self.prefix

    def get_marker(self):
        return self.marker

    def get_maxkeys(self):
        return self.maxkeys
    
    def get_istruncated(self):
        return self.is_truncated

    def get_nextmarker(self):
        return self.nextmarker
    
class Content:
    def __init__(self, xml_element):
        self.element = xml_element
        self.key = get_tag_text(self.element, "Key", convert_to_bool = False)        
        self.last_modified = get_tag_text(self.element, "LastModified")        
        self.etag = get_tag_text(self.element, "ETag")        
        self.size = get_tag_text(self.element, "Size")        
        self.owner = Owner(self.element.getElementsByTagName('Owner')[0])
        self.storage_class = get_tag_text(self.element, "StorageClass")        

    def show(self):
        print "Key: %s\nLastModified: %s\nETag: %s\nSize: %s\nStorageClass: %s" % (self.key, self.last_modified, self.etag, self.size, self.storage_class)
        self.owner.show()

class Part:
    def __init__(self, xml_element):
        self.element = xml_element
        self.part_num = get_tag_text(self.element, "PartNumber")        
        self.object_name = get_tag_text(self.element, "PartName")
        self.object_size = get_tag_text(self.element, "PartSize")
        self.etag = get_tag_text(self.element, "ETag")

    def show(self):
        print "PartNumber: %s\nPartName: %s\nPartSize: %s\nETag: %s\n" % (self.part_num, self.object_name, self.object_size, self.etag)

class PostObjectGroupXml:
    def __init__(self, xml_string):
        self.xml = minidom.parseString(xml_string)
        self.bucket = get_tag_text(self.xml, 'Bucket', convert_to_bool = False)
        self.key = get_tag_text(self.xml, 'Key', convert_to_bool = False)
        self.size = get_tag_text(self.xml, 'Size')
        self.etag = get_tag_text(self.xml, "ETag")

    def show(self):
        print "Post Object Group, Bucket: %s\nKey: %s\nSize: %s\nETag: %s" % (self.bucket, self.key, self.size, self.etag)

class GetObjectGroupIndexXml:
    def __init__(self, xml_string):
        self.xml = minidom.parseString(xml_string)
        self.bucket = get_tag_text(self.xml, 'Bucket', convert_to_bool = False)
        self.key = get_tag_text(self.xml, 'Key', convert_to_bool = False)
        self.etag = get_tag_text(self.xml, 'Etag')
        self.file_length = get_tag_text(self.xml, 'FileLength')
        self.index_list = []
        index_lists = self.xml.getElementsByTagName('Part')
        for i in index_lists:
            self.index_list.append(Part(i))

    def list(self):
        index_list = []
        for i in self.index_list:
            index_list.append((i.part_num, i.object_name, i.object_size, i.etag))
        return index_list

    def show(self):
        print "Bucket: %s\nObject: %s\nEtag: %s\nObjectSize: %s" % (self.bucket, self.key, self.etag, self.file_length)
        print "\nPart list:"
        for p in self.index_list:
            p.show()

class GetObjectLinkIndexXml:
    def __init__(self, xml_string):
        self.xml = minidom.parseString(xml_string)
        self.index_list = []
        index_lists = self.xml.getElementsByTagName('Part')
        for i in index_lists:
            self.index_list.append(Part(i))

    def list(self):
        index_list = []
        for i in self.index_list:
            index_list.append((i.part_num, i.object_name))
        return index_list

    def show(self):
        print "\nPart list:"
        for p in self.index_list:
            p.show()

class GetObjectLinkInfoXml:
    def __init__(self, xml_string):
        self.xml = minidom.parseString(xml_string)
        self.bucket = get_tag_text(self.xml, 'Bucket', convert_to_bool = False)
        self.type = get_tag_text(self.xml, 'Type')
        self.key = get_tag_text(self.xml, 'Key', convert_to_bool = False)
        self.etag = get_tag_text(self.xml, 'ETag')
        self.last_modified = get_tag_text(self.xml, 'LastModified')
        self.index_list = []
        index_lists = self.xml.getElementsByTagName('Part')
        for i in index_lists:
            self.index_list.append(Part(i))

    def list(self):
        index_list = []
        for i in self.index_list:
            index_list.append((i.part_num, i.object_name, i.object_size, i.etag))
        return index_list

    def show(self):
        print "Bucket: %s\nType: %s\nObject: %s\nEtag: %s\nLastModified: %s" % (self.bucket, self.type, self.key, self.etag, self.last_modified)
        print "\nPart list:"
        for p in self.index_list:
            p.show()

class GetBucketXml:
    def __init__(self, xml_string):
        self.xml_unescape_table = {}
        self.xml_map = {}
        try:
            self.xml = minidom.parseString(xml_string)
        except:
            print xml_string
            self.xml_unescape_tabl = get_xml_unescape_table()
            self.xml_map = get_xml_unescape_map()
            for k, v in self.xml_map.items():
                xml_string = xml_string.replace(k, v)
            self.xml = minidom.parseString(xml_string)
        
        self.name = get_tag_text(self.xml, 'Name', convert_to_bool = False)
        self.prefix = get_tag_text(self.xml, 'Prefix', convert_to_bool = False)
        self.marker = get_tag_text(self.xml, 'Marker', convert_to_bool = False)
        self.nextmarker = get_tag_text(self.xml, 'NextMarker', convert_to_bool = False)
        self.maxkeys = get_tag_text(self.xml, 'MaxKeys')
        self.delimiter = get_tag_text(self.xml, 'Delimiter', convert_to_bool = False)
        self.is_truncated = get_tag_text(self.xml, 'IsTruncated')

        self.prefix_list = []
        prefixes = self.xml.getElementsByTagName('CommonPrefixes')
        for p in prefixes:
            tag_txt = get_tag_text(p, "Prefix")
            self.prefix_list.append(tag_txt)

        self.content_list = []
        contents = self.xml.getElementsByTagName('Contents')
        for c in contents:
            self.content_list.append(Content(c))

    def show(self):
        print "Name: %s\nPrefix: %s\nMarker: %s\nNextMarker: %s\nMaxKeys: %s\nDelimiter: %s\nIsTruncated: %s" % (self.name, self.prefix, self.marker, self.nextmarker, self.maxkeys, self.delimiter, self.is_truncated)
        print "\nPrefix list:"
        for p in self.prefix_list:
            print p
        print "\nContent list:"
        for c in self.content_list:
            c.show()
            print ""

    def list(self):
        cl = []
        pl = []
        for c in self.content_list:
            key = c.key
            if self.xml_map:
                for k, v in self.xml_map.items():
                    key = key.replace(v, k)
                key = unescape(key, self.xml_unescape_table)
            cl.append((key, c.last_modified, c.etag, c.size, c.owner.id, c.owner.display_name, c.storage_class))
        for p in self.prefix_list:
            pl.append(p)

        return (cl, pl)
 
class GetBucketAclXml:
    def __init__(self, xml_string):
        self.xml = minidom.parseString(xml_string)
        if len(self.xml.getElementsByTagName('Owner')) != 0:
            self.owner = Owner(self.xml.getElementsByTagName('Owner')[0])
        else:
            self.owner = "" 
        self.grant = get_tag_text(self.xml, 'Grant')

    def show(self):
        print "Owner Name: %s\nOwner ID: %s\nGrant: %s" % (self.owner.id, self.owner.display_name, self.grant)
 
class GetBucketLocationXml:
    def __init__(self, xml_string):
        self.xml = minidom.parseString(xml_string)
        self.location = get_tag_text(self.xml, 'LocationConstraint')
    
    def show(self):
        print "LocationConstraint: %s" % (self.location)

class GetInitUploadIdXml:
    def __init__(self, xml_string):
        self.xml_unescape_table = {}
        self.xml_map = {}
        try:
            self.xml = minidom.parseString(xml_string)
        except:
            print xml_string
            self.xml_unescape_tabl = get_xml_unescape_table()
            self.xml_map = get_xml_unescape_map()
            for k, v in self.xml_map.items():
                xml_string = xml_string.replace(k, v)
            self.xml = minidom.parseString(xml_string)
        self.bucket = get_tag_text(self.xml, 'Bucket', convert_to_bool = False)
        self.object = get_tag_text(self.xml, 'Key', convert_to_bool = False)
        if self.xml_map:
            for k, v in self.xml_map.items():
                self.object = self.object.replace(v, k)
            self.object = unescape(self.object, self.xml_unescape_table)
        self.key = get_tag_text(self.xml, 'Key', convert_to_bool = False)
        self.upload_id = get_tag_text(self.xml, 'UploadId')
        self.marker = get_tag_text(self.xml, 'Marker', convert_to_bool = False)
       
    def show(self):
        print " "     

class Upload:
    def __init__(self, xml_element):
        self.element = xml_element
        self.key = get_tag_text(self.element, "Key", convert_to_bool = False)        
        self.upload_id = get_tag_text(self.element, "UploadId")
        self.init_time = get_tag_text(self.element, "Initiated")

class GetMultipartUploadsXml:
    def __init__(self, xml_string):
        self.xml_unescape_table = {} 
        self.xml_map = {}
        try:
            self.xml = minidom.parseString(xml_string)
        except:
            self.xml_unescape_tabl = get_xml_unescape_table()
            self.xml_map = get_xml_unescape_map()
            for k, v in self.xml_map.items():
                xml_string = xml_string.replace(k, v)
            self.xml = minidom.parseString(xml_string)
        
        self.bucket = get_tag_text(self.xml, 'Bucket', convert_to_bool = False)
        self.key_marker = get_tag_text(self.xml, 'KeyMarker', convert_to_bool = False)
        self.upload_id_marker = get_tag_text(self.xml, 'UploadIdMarker')
        self.next_key_marker = get_tag_text(self.xml, 'NextKeyMarker', convert_to_bool = False)
        self.next_upload_id_marker = get_tag_text(self.xml, 'NextUploadIdMarker')
        self.delimiter = get_tag_text(self.xml, 'Delimiter', convert_to_bool = False)
        self.prefix = get_tag_text(self.xml, 'Prefix', convert_to_bool = False)
        self.max_uploads = get_tag_text(self.xml, 'MaxUploads')
        self.is_truncated = get_tag_text(self.xml, 'IsTruncated')

        self.prefix_list = []
        prefixes = self.xml.getElementsByTagName('CommonPrefixes')
        for p in prefixes:
            tag_txt = get_tag_text(p, "Prefix")
            self.prefix_list.append(tag_txt)

        self.content_list = []
        contents = self.xml.getElementsByTagName('Upload')
        for c in contents:
            self.content_list.append(Upload(c))

    def list(self):
        cl = []
        pl = []
        for c in self.content_list:
            key = c.key
            if self.xml_map:
                for k, v in self.xml_map.items():
                    key = key.replace(v, k)
                key = unescape(key, self.xml_unescape_table)
            cl.append((key, c.upload_id, c.init_time))
        for p in self.prefix_list:
            pl.append(p)

        return (cl, pl)

class MultiPart:
    def __init__(self, xml_element):
        self.element = xml_element
        self.part_number = get_tag_text(self.element, 'PartNumber')
        self.last_modified = get_tag_text(self.element, 'LastModified')
        self.etag = get_tag_text(self.element, 'ETag')
        self.size = get_tag_text(self.element, 'Size')

class GetPartsXml:
    def __init__(self, xml_string):
        self.xml = minidom.parseString(xml_string)
        self.bucket = get_tag_text(self.xml, 'Bucket', convert_to_bool = False)
        self.key = get_tag_text(self.xml, 'Key', convert_to_bool = False)
        self.upload_id = get_tag_text(self.xml, 'UploadId')
        self.storage_class = get_tag_text(self.xml, 'StorageClass')
        self.next_part_number_marker = get_tag_text(self.xml, 'NextPartNumberMarker')
        self.max_parts = get_tag_text(self.xml, 'MaxParts')
        self.is_truncated = get_tag_text(self.xml, 'IsTruncated')
        self.part_number_marker = get_tag_text(self.xml, 'PartNumberMarker')
        
        self.content_list = []
        contents = self.xml.getElementsByTagName('Part')
        for c in contents:
            self.content_list.append(MultiPart(c))

    def list(self):
        cl = []
        for c in self.content_list:
            cl.append((c.part_number, c.etag, c.size, c.last_modified))
        return cl

class CompleteUploadXml:
    def __init__(self, xml_string):
        self.xml = minidom.parseString(xml_string)
        self.location = get_tag_text(self.xml, 'Location')
        self.bucket = get_tag_text(self.xml, 'Bucket', convert_to_bool = False)
        self.key = get_tag_text(self.xml, 'Key', convert_to_bool = False)
        self.etag = get_tag_text(self.xml, "ETag")

class DeletedObjectsXml:
    def __init__(self, xml_string):
        self.xml = minidom.parseString(xml_string)
        contents = self.xml.getElementsByTagName('Deleted')
        self.content_list = []
        for c in contents:
            self.content_list.append(get_tag_text(c, 'Key', convert_to_bool = False))
    def list(self):
        cl = []
        for c in self.content_list:
            cl.append(c)
        return cl

class CnameInfoPart:
    def __init__(self, xml_element):
        self.element = xml_element
        self.cname = get_tag_text(self.element, 'Cname')
        self.bucket = get_tag_text(self.element, 'Bucket', convert_to_bool = False)
        self.status = get_tag_text(self.element, 'Status')
        self.lastmodifytime = get_tag_text(self.element, 'LastModifyTime')

class CnameToBucketXml:
    def __init__(self, xml_string):
        self.xml = minidom.parseString(xml_string)
        self.content_list = []
        contents = self.xml.getElementsByTagName('CnameInfo')
        for c in contents:
            self.content_list.append(CnameInfoPart(c))

    def list(self):
        cl = []
        for c in self.content_list:
            cl.append((c.cname, c.bucket, c.status, c.lastmodifytime))
        return cl

class RedirectXml:
    def __init__(self, xml_string):
        self.xml = minidom.parseString(xml_string)
        self.endpoint = get_tag_text(self.xml, 'Endpoint')
    def Endpoint(self):
        return self.endpoint

class PostObjectResponseXml:
    def __init__(self, xml_string):
        self.xml = minidom.parseString(xml_string)
        self.bucket = get_tag_text(self.xml, 'Bucket', convert_to_bool = False)
        self.key = get_tag_text(self.xml, 'Key', convert_to_bool = False)
        self.object= get_tag_text(self.xml, 'Key', convert_to_bool = False)
        self.etag = get_tag_text(self.xml, 'ETag')
        self.location = get_tag_text(self.xml, 'Location')

    def show(self):
        print "Bucket: %s\nObject: %s\nEtag: %s\nLocation: %s" % (self.bucket, self.key, self.etag, self.location)

class WebsiteXml:
    def __init__(self, xml_element):
        self.element = minidom.parseString(xml_element) 
        self.index_file = get_tag_text(self.element, 'Suffix', convert_to_bool = False)
        self.error_file = get_tag_text(self.element, 'Key', convert_to_bool = False)

class Rule:
    def __init__(self, xml_element):
        self.max_age = ""
        self.max_age = get_tag_text(xml_element, 'MaxAgeSeconds')
        def get_list_by_tag(xml_element, tag):
            list = []
            nodes = xml_element.getElementsByTagName(tag)
            for node in nodes:
                for tmp_node in node.childNodes:
                    if tmp_node.nodeType in (tmp_node.TEXT_NODE, tmp_node.CDATA_SECTION_NODE):
                        list.append(tmp_node.data)
            return list
        self.method_list = get_list_by_tag(xml_element, 'AllowedMethod')
        self.origin_list = get_list_by_tag(xml_element, 'AllowedOrigin') 
        self.header_list = get_list_by_tag(xml_element, 'AllowedHeader')
        self.expose_header_list = get_list_by_tag(xml_element, 'ExposeHeader') 

    def show(self):
        print "max_age:%s" % self.max_age
        print "method_list:"
        for i in self.method_list:
            print "%s" % i
        print "origin_list:"
        for i in self.origin_list:
            print "%s" % i
        print "header_list:"
        for i in self.header_list:
            print "%s" % i
        print "expose_header_list:"
        for i in self.expose_header_list:
            print "%s" % i
    def get_msg(self):
        msg = "max_age:%s" % self.max_age
        msg += "method_list:"
        for i in sorted(self.method_list):
            msg += "%s" % i
        msg += "origin_list:"
        for i in sorted(self.origin_list):
            msg += "%s" % i
        msg += "header_list:"
        for i in sorted(self.header_list):
            msg += "%s" % i
        msg += "expose_header_list:"
        for i in sorted(self.expose_header_list):
            msg += "%s" % i
        return msg

class CorsXml:
    def __init__(self, xml_string):
        self.xml = minidom.parseString(xml_string)
        rules = self.xml.getElementsByTagName('CORSRule')
        self.rule_list = []
        for rule in rules:
            self.rule_list.append(Rule(rule))

class RefererXml:
    def __init__(self, xml_string):
        self.xml = minidom.parseString(xml_string)
        self.allow_empty_referer = get_tag_text(self.xml, "AllowEmptyReferer")
        self.referer_list = []
        referers = self.xml.getElementsByTagName('RefererList')[0]
        names = referers.getElementsByTagName('Referer')
        for name in names:
            for child in name.childNodes:
                r = child.nodeValue
                self.referer_list.append(r)

class LoggingXml:
    def __init__(self, xml_string):
        self.xml = minidom.parseString(xml_string)
        self.target_bucket = get_tag_text(self.xml, "TargetBucket")
        self.prefix = get_tag_text(self.xml, "TargetPrefix")

class LifecycleRule:
    def __init__(self, xml_element):
        self.element = xml_element
        self.id = get_tag_text(self.element, "ID")
        self.prefix = get_tag_text(self.element, "Prefix")
        self.status = get_tag_text(self.element, "Status")
        self.date = get_tag_text(self.element, "Date", '')
        self.days = get_tag_text(self.element, "Days", '')
    
    def show(self):
        print "ID: %s\nPrefix: %s\nStatus: %s\nExpiration: %s\n" % (self.id, self.prefix, self.status, self.expiration)

class LifecycleXml:
    def __init__(self, xml_string):
        self.xml = minidom.parseString(xml_string)
        self.rules = self.xml.getElementsByTagName('Rule')
        self.rule_list = []
        for r in self.rules:
            self.rule_list.append(LifecycleRule(r))

    def show(self):
        print "\nRule list:"
        for b in self.rule_list:
            b.show()

class GetObjectInfoXml:
    def __init__(self, xml_string):
        try:
            self.xml = minidom.parseString(xml_string)
        except:
            print xml_string
        self.bucket = get_tag_text(self.xml, 'Bucket', convert_to_bool = False)
        self.type = get_tag_text(self.xml, 'Type', convert_to_bool = False)
        self.key = get_tag_text(self.xml, 'Key', convert_to_bool = False)
        self.last_modified = get_tag_text(self.xml, 'LastModified', convert_to_bool = False)
        self.etag = get_tag_text(self.xml, 'ETag', convert_to_bool = False)
        self.content_type = get_tag_text(self.xml, 'Content-Type')
        self.size = get_tag_text(self.xml, 'Size', convert_to_bool = False)
        self.parts = []
        parts = self.xml.getElementsByTagName('Part')
        for p in parts:
            self.parts.append(Part(p))

    def show(self):
        print "Bucket: %s\nType: %s\nKey: %s\nLastModified: %s\nETag: %s\nContent-Type: %s\nSize: %s" % (self.bucket, self.type, self.key, self.last_modified, self.etag, self.content_type, self.size)

if __name__ == "__main__":
    pass
