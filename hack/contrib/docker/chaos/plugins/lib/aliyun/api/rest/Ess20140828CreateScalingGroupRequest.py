'''
Created by auto_sdk on 2015.04.21
'''
from aliyun.api.base import RestApi
class Ess20140828CreateScalingGroupRequest(RestApi):
	def __init__(self,domain='ess.aliyuncs.com',port=80):
		RestApi.__init__(self,domain, port)
		self.DBInstanceId_1 = None
		self.DBInstanceId_2 = None
		self.DBInstanceId_3 = None
		self.DefaultCooldown = None
		self.LoadBalancerId = None
		self.MaxSize = None
		self.MinSize = None
		self.RegionId = None
		self.RemovalPolicy_1 = None
		self.RemovalPolicy_2 = None
		self.ScalingGroupName = None

	def getapiname(self):
		return 'ess.aliyuncs.com.CreateScalingGroup.2014-08-28'

	def getTranslateParas(self):
		return {'DBInstanceId_3':'DBInstanceId.3','RemovalPolicy_1':'RemovalPolicy.1','DBInstanceId_2':'DBInstanceId.2','RemovalPolicy_2':'RemovalPolicy.2','DBInstanceId_1':'DBInstanceId.1'}
