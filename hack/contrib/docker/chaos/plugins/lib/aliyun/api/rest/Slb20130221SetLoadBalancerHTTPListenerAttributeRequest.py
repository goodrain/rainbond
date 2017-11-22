'''
Created by auto_sdk on 2015.04.21
'''
from aliyun.api.base import RestApi
class Slb20130221SetLoadBalancerHTTPListenerAttributeRequest(RestApi):
	def __init__(self,domain='slb.aliyuncs.com',port=80):
		RestApi.__init__(self,domain, port)
		self.cookie = None
		self.cookieTimeout = None
		self.domain = None
		self.healthCheck = None
		self.healthCheckTimeout = None
		self.healthyThreshold = None
		self.interval = None
		self.listenerPort = None
		self.loadBalancerId = None
		self.scheduler = None
		self.stickySession = None
		self.stickySessionType = None
		self.unhealthyThreshold = None
		self.uri = None
		self.xForwardedFor = None

	def getapiname(self):
		return 'slb.aliyuncs.com.SetLoadBalancerHTTPListenerAttribute.2013-02-21'
