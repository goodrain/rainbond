'''
Created by auto_sdk on 2015.04.21
'''
from aliyun.api.base import RestApi
class Slb20140515RemoveBackendServersRequest(RestApi):
	def __init__(self,domain='slb.aliyuncs.com',port=80):
		RestApi.__init__(self,domain, port)
		self.BackendServers = None
		self.LoadBalancerId = None

	def getapiname(self):
		return 'slb.aliyuncs.com.RemoveBackendServers.2014-05-15'
