'''
Created by auto_sdk on 2015.04.21
'''
from aliyun.api.base import RestApi
class Ecs20140526CreateRouteEntryRequest(RestApi):
	def __init__(self,domain='ecs.aliyuncs.com',port=80):
		RestApi.__init__(self,domain, port)
		self.ClientToken = None
		self.DestinationCidrBlock = None
		self.NextHopId = None
		self.NextHopType = None
		self.RouteTableId = None

	def getapiname(self):
		return 'ecs.aliyuncs.com.CreateRouteEntry.2014-05-26'
