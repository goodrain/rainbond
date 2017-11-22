'''
Created by auto_sdk on 2015.04.21
'''
from aliyun.api.base import RestApi
class Ecs20140526DescribeImageSharePermissionRequest(RestApi):
	def __init__(self,domain='ecs.aliyuncs.com',port=80):
		RestApi.__init__(self,domain, port)
		self.ImageId = None
		self.PageNumber = None
		self.PageSize = None
		self.RegionId = None

	def getapiname(self):
		return 'ecs.aliyuncs.com.DescribeImageSharePermission.2014-05-26'
