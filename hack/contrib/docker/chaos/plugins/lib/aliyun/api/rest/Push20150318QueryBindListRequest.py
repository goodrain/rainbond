'''
Created by auto_sdk on 2015.04.21
'''
from aliyun.api.base import RestApi
class Push20150318QueryBindListRequest(RestApi):
	def __init__(self,domain='push.aliyuncs.com',port=80):
		RestApi.__init__(self,domain, port)
		self.Account = None
		self.AppId = None
		self.DeviceType = None

	def getapiname(self):
		return 'push.aliyuncs.com.queryBindList.2015-03-18'
