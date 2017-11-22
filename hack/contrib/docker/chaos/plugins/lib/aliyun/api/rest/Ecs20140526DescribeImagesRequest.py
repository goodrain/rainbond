'''
Created by auto_sdk on 2015.04.21
'''
from aliyun.api.base import RestApi
class Ecs20140526DescribeImagesRequest(RestApi):
	def __init__(self,domain='ecs.aliyuncs.com',port=80):
		RestApi.__init__(self,domain, port)
		self.Architecture = None
		self.ImageId = None
		self.ImageName = None
		self.ImageOwnerAlias = None
		self.PageNumber = None
		self.PageSize = None
		self.RegionId = None
		self.SnapshotId = None
		self.Status = None

	def getapiname(self):
		return 'ecs.aliyuncs.com.DescribeImages.2014-05-26'
