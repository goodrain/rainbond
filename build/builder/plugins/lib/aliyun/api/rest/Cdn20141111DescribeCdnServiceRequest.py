'''
Created by auto_sdk on 2015.04.21
'''
from aliyun.api.base import RestApi
class Cdn20141111DescribeCdnServiceRequest(RestApi):
	def __init__(self,domain='cdn.aliyuncs.com',port=80):
		RestApi.__init__(self,domain, port)

	def getapiname(self):
		return 'cdn.aliyuncs.com.DescribeCdnService.2014-11-11'
