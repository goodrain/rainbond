'''
Created by auto_sdk on 2015.04.21
'''
from aliyun.api.base import RestApi
class Ess20140828DescribeScalingConfigurationsRequest(RestApi):
	def __init__(self,domain='ess.aliyuncs.com',port=80):
		RestApi.__init__(self,domain, port)
		self.PageNumber = None
		self.PageSize = None
		self.RegionId = None
		self.ScalingConfigurationId_1 = None
		self.ScalingConfigurationId_10 = None
		self.ScalingConfigurationId_2 = None
		self.ScalingConfigurationId_3 = None
		self.ScalingConfigurationId_4 = None
		self.ScalingConfigurationId_5 = None
		self.ScalingConfigurationId_6 = None
		self.ScalingConfigurationId_7 = None
		self.ScalingConfigurationId_8 = None
		self.ScalingConfigurationId_9 = None
		self.ScalingConfigurationName_1 = None
		self.ScalingConfigurationName_10 = None
		self.ScalingConfigurationName_2 = None
		self.ScalingConfigurationName_3 = None
		self.ScalingConfigurationName_4 = None
		self.ScalingConfigurationName_5 = None
		self.ScalingConfigurationName_6 = None
		self.ScalingConfigurationName_7 = None
		self.ScalingConfigurationName_8 = None
		self.ScalingConfigurationName_9 = None
		self.ScalingGroupId = None

	def getapiname(self):
		return 'ess.aliyuncs.com.DescribeScalingConfigurations.2014-08-28'

	def getTranslateParas(self):
		return {'ScalingConfigurationName_8':'ScalingConfigurationName.8','ScalingConfigurationId_1':'ScalingConfigurationId.1','ScalingConfigurationName_7':'ScalingConfigurationName.7','ScalingConfigurationId_2':'ScalingConfigurationId.2','ScalingConfigurationName_6':'ScalingConfigurationName.6','ScalingConfigurationId_3':'ScalingConfigurationId.3','ScalingConfigurationName_5':'ScalingConfigurationName.5','ScalingConfigurationId_4':'ScalingConfigurationId.4','ScalingConfigurationName_9':'ScalingConfigurationName.9','ScalingConfigurationId_10':'ScalingConfigurationId.10','ScalingConfigurationName_10':'ScalingConfigurationName.10','ScalingConfigurationId_7':'ScalingConfigurationId.7','ScalingConfigurationId_8':'ScalingConfigurationId.8','ScalingConfigurationId_5':'ScalingConfigurationId.5','ScalingConfigurationId_6':'ScalingConfigurationId.6','ScalingConfigurationId_9':'ScalingConfigurationId.9','ScalingConfigurationName_3':'ScalingConfigurationName.3','ScalingConfigurationName_4':'ScalingConfigurationName.4','ScalingConfigurationName_1':'ScalingConfigurationName.1','ScalingConfigurationName_2':'ScalingConfigurationName.2'}
