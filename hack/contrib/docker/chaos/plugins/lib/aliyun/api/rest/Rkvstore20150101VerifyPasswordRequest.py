'''
Created by auto_sdk on 2015.04.21
'''
from aliyun.api.base import RestApi
class Rkvstore20150101VerifyPasswordRequest(RestApi):
	def __init__(self,domain='r-kvstore.aliyuncs.com',port=80):
		RestApi.__init__(self,domain, port)
		self.InstanceId = None
		self.Password = None

	def getapiname(self):
		return 'r-kvstore.aliyuncs.com.VerifyPassword.2015-01-01'
