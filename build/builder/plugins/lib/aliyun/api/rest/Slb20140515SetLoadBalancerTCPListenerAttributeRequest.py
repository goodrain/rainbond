'''
Created by auto_sdk on 2015.04.21
'''
from aliyun.api.base import RestApi
class Slb20140515SetLoadBalancerTCPListenerAttributeRequest(RestApi):
	def __init__(self,domain='slb.aliyuncs.com',port=80):
		RestApi.__init__(self,domain, port)
		self.Bandwidth = None
		self.HealthCheckConnectPort = None
		self.HealthCheckConnectTimeout = None
		self.HealthCheckInterval = None
		self.HealthyThreshold = None
		self.ListenerPort = None
		self.LoadBalancerId = None
		self.PersistenceTimeout = None
		self.Scheduler = None
		self.UnhealthyThreshold = None

	def getapiname(self):
		return 'slb.aliyuncs.com.SetLoadBalancerTCPListenerAttribute.2014-05-15'
