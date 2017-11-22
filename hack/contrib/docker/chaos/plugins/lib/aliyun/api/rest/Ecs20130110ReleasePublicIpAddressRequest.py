'''
Created by auto_sdk on 2015.04.21
'''
from aliyun.api.base import RestApi
class Ecs20130110ReleasePublicIpAddressRequest(RestApi):
	def __init__(self,domain='ecs.aliyuncs.com',port=80):
		RestApi.__init__(self,domain, port)
		self.InstanceId = None
		self.PublicIpAddress = None

	def getapiname(self):
		return 'ecs.aliyuncs.com.ReleasePublicIpAddress.2013-01-10'
