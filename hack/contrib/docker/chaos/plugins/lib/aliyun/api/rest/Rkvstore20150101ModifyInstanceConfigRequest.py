'''
Created by auto_sdk on 2015.04.21
'''
from aliyun.api.base import RestApi
class Rkvstore20150101ModifyInstanceConfigRequest(RestApi):
	def __init__(self,domain='r-kvstore.aliyuncs.com',port=80):
		RestApi.__init__(self,domain, port)
		self.Config = None
		self.InstanceId = None

	def getapiname(self):
		return 'r-kvstore.aliyuncs.com.ModifyInstanceConfig.2015-01-01'
