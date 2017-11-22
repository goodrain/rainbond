'''
Created by auto_sdk on 2015.04.21
'''
from aliyun.api.base import RestApi
class Cdn20141111DescribeUserDomainsRequest(RestApi):
	def __init__(self,domain='cdn.aliyuncs.com',port=80):
		RestApi.__init__(self,domain, port)
		self.PageNumber = None
		self.PageSize = None

	def getapiname(self):
		return 'cdn.aliyuncs.com.DescribeUserDomains.2014-11-11'
