'''
Created by auto_sdk on 2015.04.21
'''
from aliyun.api.base import RestApi
class Dns20150109DescribeDomainRecordsRequest(RestApi):
	def __init__(self,domain='dns.aliyuncs.com',port=80):
		RestApi.__init__(self,domain, port)
		self.DomainName = None
		self.PageNumber = None
		self.PageSize = None
		self.RRKeyWord = None
		self.TypeKeyWord = None
		self.ValueKeyWord = None

	def getapiname(self):
		return 'dns.aliyuncs.com.DescribeDomainRecords.2015-01-09'
