'''
Created by auto_sdk on 2015.04.21
'''
from aliyun.api.base import RestApi
class Slb20140515SetLoadBalancerHTTPListenerAttributeRequest(RestApi):
	def __init__(self,domain='slb.aliyuncs.com',port=80):
		RestApi.__init__(self,domain, port)
		self.Bandwidth = None
		self.Cookie = None
		self.CookieTimeout = None
		self.HealthCheck = None
		self.HealthCheckConnectPort = None
		self.HealthCheckDomain = None
		self.HealthCheckHttpCode = None
		self.HealthCheckInterval = None
		self.HealthCheckTimeout = None
		self.HealthCheckURI = None
		self.HealthyThreshold = None
		self.ListenerPort = None
		self.LoadBalancerId = None
		self.Scheduler = None
		self.StickySession = None
		self.StickySessionType = None
		self.UnhealthyThreshold = None
		self.XForwardedFor = None

	def getapiname(self):
		return 'slb.aliyuncs.com.SetLoadBalancerHTTPListenerAttribute.2014-05-15'
