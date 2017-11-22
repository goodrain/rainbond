'''
Created by auto_sdk on 2015.04.21
'''
from aliyun.api.base import RestApi
class Cdn20141111RefreshObjectCachesRequest(RestApi):
	def __init__(self,domain='cdn.aliyuncs.com',port=80):
		RestApi.__init__(self,domain, port)
		self.ObjectPath = None
		self.ObjectType = None

	def getapiname(self):
		return 'cdn.aliyuncs.com.RefreshObjectCaches.2014-11-11'
