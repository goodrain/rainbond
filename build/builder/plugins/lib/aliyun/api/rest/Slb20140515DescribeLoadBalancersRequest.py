'''
Created by auto_sdk on 2015.04.21
'''
from aliyun.api.base import RestApi
class Slb20140515DescribeLoadBalancersRequest(RestApi):
	def __init__(self,domain='slb.aliyuncs.com',port=80):
		RestApi.__init__(self,domain, port)
		self.Address = None
		self.AddressType = None
		self.InternetChargeType = None
		self.LoadBalancerId = None
		self.NetworkType = None
		self.RegionId = None
		self.ServerId = None
		self.VSwitchId = None
		self.VpcId = None

	def getapiname(self):
		return 'slb.aliyuncs.com.DescribeLoadBalancers.2014-05-15'
