'''
Created by auto_sdk on 2015.04.21
'''
from aliyun.api.base import RestApi
class Dns20150109DescribeDomainWhoisInfoRequest(RestApi):
	def __init__(self,domain='dns.aliyuncs.com',port=80):
		RestApi.__init__(self,domain, port)
		self.DomainName = None

	def getapiname(self):
		return 'dns.aliyuncs.com.DescribeDomainWhoisInfo.2015-01-09'
