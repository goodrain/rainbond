'''
Created by auto_sdk on 2015.04.21
'''
from aliyun.api.base import RestApi
class Ess20140828DescribeScalingInstancesRequest(RestApi):
	def __init__(self,domain='ess.aliyuncs.com',port=80):
		RestApi.__init__(self,domain, port)
		self.CreationType = None
		self.HealthStatus = None
		self.InstanceId_1 = None
		self.InstanceId_10 = None
		self.InstanceId_11 = None
		self.InstanceId_12 = None
		self.InstanceId_13 = None
		self.InstanceId_14 = None
		self.InstanceId_15 = None
		self.InstanceId_16 = None
		self.InstanceId_17 = None
		self.InstanceId_18 = None
		self.InstanceId_19 = None
		self.InstanceId_2 = None
		self.InstanceId_20 = None
		self.InstanceId_3 = None
		self.InstanceId_4 = None
		self.InstanceId_5 = None
		self.InstanceId_6 = None
		self.InstanceId_7 = None
		self.InstanceId_8 = None
		self.InstanceId_9 = None
		self.LifecycleState = None
		self.PageNumber = None
		self.PageSize = None
		self.RegionId = None
		self.ScalingConfigurationId = None
		self.ScalingGroupId = None

	def getapiname(self):
		return 'ess.aliyuncs.com.DescribeScalingInstances.2014-08-28'

	def getTranslateParas(self):
		return {'InstanceId_19':'InstanceId.19','InstanceId_18':'InstanceId.18','InstanceId_17':'InstanceId.17','InstanceId_16':'InstanceId.16','InstanceId_15':'InstanceId.15','InstanceId_14':'InstanceId.14','InstanceId_13':'InstanceId.13','InstanceId_12':'InstanceId.12','InstanceId_10':'InstanceId.10','InstanceId_11':'InstanceId.11','InstanceId_20':'InstanceId.20','InstanceId_9':'InstanceId.9','InstanceId_8':'InstanceId.8','InstanceId_7':'InstanceId.7','InstanceId_6':'InstanceId.6','InstanceId_5':'InstanceId.5','InstanceId_4':'InstanceId.4','InstanceId_3':'InstanceId.3','InstanceId_2':'InstanceId.2','InstanceId_1':'InstanceId.1'}
