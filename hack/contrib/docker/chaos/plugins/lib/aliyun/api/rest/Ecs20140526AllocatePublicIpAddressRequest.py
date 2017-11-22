'''
Created by auto_sdk on 2015.04.21
'''
from aliyun.api.base import RestApi
class Ecs20140526AllocatePublicIpAddressRequest(RestApi):
	def __init__(self,domain='ecs.aliyuncs.com',port=80):
		RestApi.__init__(self,domain, port)
		self.InstanceId = None
		self.IpAddress = None
		self.VlanId = None

	def getapiname(self):
		return 'ecs.aliyuncs.com.AllocatePublicIpAddress.2014-05-26'
