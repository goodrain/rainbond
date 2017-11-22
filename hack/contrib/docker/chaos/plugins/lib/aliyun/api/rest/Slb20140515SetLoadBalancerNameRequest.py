'''
Created by auto_sdk on 2015.04.21
'''
from aliyun.api.base import RestApi
class Slb20140515SetLoadBalancerNameRequest(RestApi):
	def __init__(self,domain='slb.aliyuncs.com',port=80):
		RestApi.__init__(self,domain, port)
		self.LoadBalancerId = None
		self.LoadBalancerName = None

	def getapiname(self):
		return 'slb.aliyuncs.com.SetLoadBalancerName.2014-05-15'
