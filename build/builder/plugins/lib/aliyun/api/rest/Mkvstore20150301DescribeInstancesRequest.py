'''
Created by auto_sdk on 2015.04.21
'''
from aliyun.api.base import RestApi
class Mkvstore20150301DescribeInstancesRequest(RestApi):
	def __init__(self,domain='m-kvstore.aliyuncs.com',port=80):
		RestApi.__init__(self,domain, port)
		self.InstanceIds = None
		self.InstanceStatus = None
		self.NetworkType = None
		self.PageNumber = None
		self.PageSize = None
		self.PrivateIpAddresses = None
		self.RegionId = None
		self.VSwitchId = None
		self.VpcId = None

	def getapiname(self):
		return 'm-kvstore.aliyuncs.com.DescribeInstances.2015-03-01'
