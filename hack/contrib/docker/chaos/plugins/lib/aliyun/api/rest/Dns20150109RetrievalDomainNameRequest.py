'''
Created by auto_sdk on 2015.04.21
'''
from aliyun.api.base import RestApi
class Dns20150109RetrievalDomainNameRequest(RestApi):
	def __init__(self,domain='dns.aliyuncs.com',port=80):
		RestApi.__init__(self,domain, port)
		self.domainName = None

	def getapiname(self):
		return 'dns.aliyuncs.com.RetrievalDomainName.2015-01-09'
