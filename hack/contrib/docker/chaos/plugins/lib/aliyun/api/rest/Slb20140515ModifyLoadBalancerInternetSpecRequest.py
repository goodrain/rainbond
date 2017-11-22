'''
Created by auto_sdk on 2015.04.21
'''
from aliyun.api.base import RestApi
class Slb20140515ModifyLoadBalancerInternetSpecRequest(RestApi):
	def __init__(self,domain='slb.aliyuncs.com',port=80):
		RestApi.__init__(self,domain, port)
		self.Bandwidth = None
		self.InternetChargeType = None
		self.LoadBalancerId = None

	def getapiname(self):
		return 'slb.aliyuncs.com.ModifyLoadBalancerInternetSpec.2014-05-15'
