'''
Created by auto_sdk on 2015.04.21
'''
from aliyun.api.base import RestApi
class Push20150318GetNotificationDetailRequest(RestApi):
	def __init__(self,domain='push.aliyuncs.com',port=80):
		RestApi.__init__(self,domain, port)
		self.AppId = None
		self.NotifyId = None

	def getapiname(self):
		return 'push.aliyuncs.com.getNotificationDetail.2015-03-18'
