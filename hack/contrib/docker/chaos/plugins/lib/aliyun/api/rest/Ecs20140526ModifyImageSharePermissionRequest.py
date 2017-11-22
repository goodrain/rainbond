'''
Created by auto_sdk on 2015.04.21
'''
from aliyun.api.base import RestApi
class Ecs20140526ModifyImageSharePermissionRequest(RestApi):
	def __init__(self,domain='ecs.aliyuncs.com',port=80):
		RestApi.__init__(self,domain, port)
		self.AddAccount_1 = None
		self.AddAccount_10 = None
		self.AddAccount_2 = None
		self.AddAccount_3 = None
		self.AddAccount_4 = None
		self.AddAccount_5 = None
		self.AddAccount_6 = None
		self.AddAccount_7 = None
		self.AddAccount_8 = None
		self.AddAccount_9 = None
		self.ImageId = None
		self.RegionId = None
		self.RemoveAccount_1 = None
		self.RemoveAccount_10 = None
		self.RemoveAccount_2 = None
		self.RemoveAccount_3 = None
		self.RemoveAccount_4 = None
		self.RemoveAccount_5 = None
		self.RemoveAccount_6 = None
		self.RemoveAccount_7 = None
		self.RemoveAccount_8 = None
		self.RemoveAccount_9 = None

	def getapiname(self):
		return 'ecs.aliyuncs.com.ModifyImageSharePermission.2014-05-26'

	def getTranslateParas(self):
		return {'RemoveAccount_9':'RemoveAccount.9','RemoveAccount_8':'RemoveAccount.8','RemoveAccount_7':'RemoveAccount.7','RemoveAccount_6':'RemoveAccount.6','AddAccount_2':'AddAccount.2','AddAccount_1':'AddAccount.1','AddAccount_7':'AddAccount.7','AddAccount_8':'AddAccount.8','AddAccount_9':'AddAccount.9','AddAccount_3':'AddAccount.3','AddAccount_4':'AddAccount.4','AddAccount_5':'AddAccount.5','AddAccount_6':'AddAccount.6','AddAccount_10':'AddAccount.10','RemoveAccount_1':'RemoveAccount.1','RemoveAccount_2':'RemoveAccount.2','RemoveAccount_3':'RemoveAccount.3','RemoveAccount_4':'RemoveAccount.4','RemoveAccount_5':'RemoveAccount.5','RemoveAccount_10':'RemoveAccount.10'}
