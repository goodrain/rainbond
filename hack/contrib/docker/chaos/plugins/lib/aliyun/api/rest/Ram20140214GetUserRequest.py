'''
Created by auto_sdk on 2015.04.21
'''
from aliyun.api.base import RestApi
class Ram20140214GetUserRequest(RestApi):
	def __init__(self,domain='ram.aliyuncs.com',port=80):
		RestApi.__init__(self,domain, port)
		self.AccountSpace = None
		self.UserName = None

	def getapiname(self):
		return 'ram.aliyuncs.com.GetUser.2014-02-14'
