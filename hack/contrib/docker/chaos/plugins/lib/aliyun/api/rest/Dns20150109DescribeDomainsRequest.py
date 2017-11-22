'''
Created by auto_sdk on 2015.04.21
'''
from aliyun.api.base import RestApi
class Dns20150109DescribeDomainsRequest(RestApi):
	def __init__(self,domain='dns.aliyuncs.com',port=80):
		RestApi.__init__(self,domain, port)
		self.KeyWord = None
		self.PageSize = None
		self.pageNumber = None

	def getapiname(self):
		return 'dns.aliyuncs.com.DescribeDomains.2015-01-09'
