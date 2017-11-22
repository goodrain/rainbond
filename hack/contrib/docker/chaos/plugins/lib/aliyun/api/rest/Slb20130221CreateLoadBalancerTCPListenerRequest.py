'''
Created by auto_sdk on 2015.04.21
'''
from aliyun.api.base import RestApi
class Slb20130221CreateLoadBalancerTCPListenerRequest(RestApi):
	def __init__(self,domain='slb.aliyuncs.com',port=80):
		RestApi.__init__(self,domain, port)
		self.backendServerPort = None
		self.connectPort = None
		self.connectTimeout = None
		self.healthCheck = None
		self.interval = None
		self.listenerPort = None
		self.listenerStatus = None
		self.loadBalancerId = None
		self.persistenceTimeout = None
		self.scheduler = None

	def getapiname(self):
		return 'slb.aliyuncs.com.CreateLoadBalancerTCPListener.2013-02-21'
