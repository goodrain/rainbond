'''
Created by auto_sdk on 2015.04.21
'''
from aliyun.api.base import RestApi
class Dns20150109DescribeDomainRecordInfoRequest(RestApi):
	def __init__(self,domain='dns.aliyuncs.com',port=80):
		RestApi.__init__(self,domain, port)
		self.RecordId = None

	def getapiname(self):
		return 'dns.aliyuncs.com.DescribeDomainRecordInfo.2015-01-09'
