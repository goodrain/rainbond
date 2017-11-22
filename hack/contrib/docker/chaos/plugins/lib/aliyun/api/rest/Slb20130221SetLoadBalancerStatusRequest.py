'''
Created by auto_sdk on 2015.04.21
'''
from aliyun.api.base import RestApi
class Slb20130221SetLoadBalancerStatusRequest(RestApi):
	def __init__(self,domain='slb.aliyuncs.com',port=80):
		RestApi.__init__(self,domain, port)
		self.loadBalancerId = None
		self.loadBalancerStatus = None

	def getapiname(self):
		return 'slb.aliyuncs.com.SetLoadBalancerStatus.2013-02-21'
