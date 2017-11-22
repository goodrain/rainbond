'''
Created by auto_sdk on 2015.04.21
'''
from aliyun.api.base import RestApi
class Ecs20140526ModifySecurityGroupAttributeRequest(RestApi):
	def __init__(self,domain='ecs.aliyuncs.com',port=80):
		RestApi.__init__(self,domain, port)
		self.Description = None
		self.RegionId = None
		self.SecurityGroupId = None
		self.SecurityGroupName = None

	def getapiname(self):
		return 'ecs.aliyuncs.com.ModifySecurityGroupAttribute.2014-05-26'
