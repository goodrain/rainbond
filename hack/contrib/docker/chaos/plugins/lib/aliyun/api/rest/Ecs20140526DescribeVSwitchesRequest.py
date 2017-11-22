'''
Created by auto_sdk on 2015.04.21
'''
from aliyun.api.base import RestApi
class Ecs20140526DescribeVSwitchesRequest(RestApi):
	def __init__(self,domain='ecs.aliyuncs.com',port=80):
		RestApi.__init__(self,domain, port)
		self.PageNumber = None
		self.PageSize = None
		self.VSwitchId = None
		self.VpcId = None
		self.ZoneId = None

	def getapiname(self):
		return 'ecs.aliyuncs.com.DescribeVSwitches.2014-05-26'
