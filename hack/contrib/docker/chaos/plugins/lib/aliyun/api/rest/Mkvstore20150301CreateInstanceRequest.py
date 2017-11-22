'''
Created by auto_sdk on 2015.04.21
'''
from aliyun.api.base import RestApi
class Mkvstore20150301CreateInstanceRequest(RestApi):
	def __init__(self,domain='m-kvstore.aliyuncs.com',port=80):
		RestApi.__init__(self,domain, port)
		self.Capacity = None
		self.InstanceName = None
		self.NetworkType = None
		self.Password = None
		self.PrivateIpAddress = None
		self.RegionId = None
		self.Token = None
		self.VSwitchId = None
		self.VpcId = None
		self.ZoneId = None

	def getapiname(self):
		return 'm-kvstore.aliyuncs.com.CreateInstance.2015-03-01'
