'''
Created by auto_sdk on 2015.04.21
'''
from aliyun.api.base import RestApi
class Ram20140214ListUsersRequest(RestApi):
	def __init__(self,domain='ram.aliyuncs.com',port=80):
		RestApi.__init__(self,domain, port)
		self.AccountSpace = None

	def getapiname(self):
		return 'ram.aliyuncs.com.ListUsers.2014-02-14'
