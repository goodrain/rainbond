'''
Created by auto_sdk on 2015.04.21
'''
from aliyun.api.base import RestApi
class Rkvstore20150101CreateInstanceRequest(RestApi):
	def __init__(self,domain='r-kvstore.aliyuncs.com',port=80):
		RestApi.__init__(self,domain, port)
		self.Capacity = None
		self.Config = None
		self.InstanceName = None
		self.Password = None
		self.RegionId = None
		self.Token = None
		self.ZoneId = None

	def getapiname(self):
		return 'r-kvstore.aliyuncs.com.CreateInstance.2015-01-01'
