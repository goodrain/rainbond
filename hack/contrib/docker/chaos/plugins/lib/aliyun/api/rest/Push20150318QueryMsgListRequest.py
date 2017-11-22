'''
Created by auto_sdk on 2015.04.21
'''
from aliyun.api.base import RestApi
class Push20150318QueryMsgListRequest(RestApi):
	def __init__(self,domain='push.aliyuncs.com',port=80):
		RestApi.__init__(self,domain, port)
		self.AppId = None
		self.Limit = None
		self.Start = None
		self.Status = None
		self.TimeEnd = None
		self.TimeStart = None
		self.Title = None

	def getapiname(self):
		return 'push.aliyuncs.com.queryMsgList.2015-03-18'
