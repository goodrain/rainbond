'''
Created by auto_sdk on 2015.04.21
'''
from aliyun.api.base import RestApi
class Mkvstore20150301DeleteInstanceRequest(RestApi):
	def __init__(self,domain='m-kvstore.aliyuncs.com',port=80):
		RestApi.__init__(self,domain, port)
		self.InstanceId = None

	def getapiname(self):
		return 'm-kvstore.aliyuncs.com.DeleteInstance.2015-03-01'
