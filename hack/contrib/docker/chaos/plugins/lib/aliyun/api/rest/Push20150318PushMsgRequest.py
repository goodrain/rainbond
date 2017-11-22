'''
Created by auto_sdk on 2015.04.21
'''
from aliyun.api.base import RestApi
class Push20150318PushMsgRequest(RestApi):
	def __init__(self,domain='push.aliyuncs.com',port=80):
		RestApi.__init__(self,domain, port)
		self.Account = None
		self.AntiHarassDuration = None
		self.AntiHarassStartTime = None
		self.AppId = None
		self.BatchNumber = None
		self.Body = None
		self.DeviceId = None
		self.DeviceType = None
		self.PushTime = None
		self.SendType = None
		self.Tag = None
		self.Timeout = None
		self.Title = None

	def getapiname(self):
		return 'push.aliyuncs.com.pushMsg.2015-03-18'
