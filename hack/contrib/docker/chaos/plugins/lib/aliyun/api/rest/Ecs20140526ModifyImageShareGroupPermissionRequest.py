'''
Created by auto_sdk on 2015.04.21
'''
from aliyun.api.base import RestApi
class Ecs20140526ModifyImageShareGroupPermissionRequest(RestApi):
	def __init__(self,domain='ecs.aliyuncs.com',port=80):
		RestApi.__init__(self,domain, port)
		self.AddGroup_1 = None
		self.ImageId = None
		self.RegionId = None
		self.RemoveGroup_1 = None

	def getapiname(self):
		return 'ecs.aliyuncs.com.ModifyImageShareGroupPermission.2014-05-26'

	def getTranslateParas(self):
		return {'RemoveGroup_1':'RemoveGroup.1','AddGroup_1':'AddGroup.1'}
