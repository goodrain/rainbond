'''
Created by auto_sdk on 2015.04.21
'''
from aliyun.api.base import RestApi
class Push20150318PushNotificationRequest(RestApi):
	def __init__(self,domain='push.aliyuncs.com',port=80):
		RestApi.__init__(self,domain, port)
		self.Account = None
		self.AndroidExtraMap = None
		self.AndroidMusic = None
		self.AndroidNotifyType = None
		self.AndroidOpenActivity = None
		self.AndroidOpenType = None
		self.AndroidOpenUrl = None
		self.AntiHarassDuration = None
		self.AntiHarassStartTime = None
		self.AppId = None
		self.BatchNumber = None
		self.DeviceId = None
		self.DeviceType = None
		self.IosExtraMap = None
		self.IosFooter = None
		self.IosMusic = None
		self.PushTime = None
		self.SendType = None
		self.Summary = None
		self.Tag = None
		self.Timeout = None
		self.Title = None

	def getapiname(self):
		return 'push.aliyuncs.com.pushNotification.2015-03-18'
