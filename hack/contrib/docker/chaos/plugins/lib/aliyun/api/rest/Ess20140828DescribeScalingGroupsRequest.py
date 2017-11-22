'''
Created by auto_sdk on 2015.04.21
'''
from aliyun.api.base import RestApi
class Ess20140828DescribeScalingGroupsRequest(RestApi):
	def __init__(self,domain='ess.aliyuncs.com',port=80):
		RestApi.__init__(self,domain, port)
		self.PageNumber = None
		self.PageSize = None
		self.RegionId = None
		self.ScalingGroupId_1 = None
		self.ScalingGroupId_10 = None
		self.ScalingGroupId_11 = None
		self.ScalingGroupId_12 = None
		self.ScalingGroupId_13 = None
		self.ScalingGroupId_14 = None
		self.ScalingGroupId_15 = None
		self.ScalingGroupId_16 = None
		self.ScalingGroupId_17 = None
		self.ScalingGroupId_18 = None
		self.ScalingGroupId_19 = None
		self.ScalingGroupId_2 = None
		self.ScalingGroupId_20 = None
		self.ScalingGroupId_3 = None
		self.ScalingGroupId_4 = None
		self.ScalingGroupId_5 = None
		self.ScalingGroupId_6 = None
		self.ScalingGroupId_7 = None
		self.ScalingGroupId_8 = None
		self.ScalingGroupId_9 = None
		self.ScalingGroupName_1 = None
		self.ScalingGroupName_10 = None
		self.ScalingGroupName_11 = None
		self.ScalingGroupName_12 = None
		self.ScalingGroupName_13 = None
		self.ScalingGroupName_14 = None
		self.ScalingGroupName_15 = None
		self.ScalingGroupName_16 = None
		self.ScalingGroupName_17 = None
		self.ScalingGroupName_18 = None
		self.ScalingGroupName_19 = None
		self.ScalingGroupName_2 = None
		self.ScalingGroupName_20 = None
		self.ScalingGroupName_3 = None
		self.ScalingGroupName_4 = None
		self.ScalingGroupName_5 = None
		self.ScalingGroupName_6 = None
		self.ScalingGroupName_7 = None
		self.ScalingGroupName_8 = None
		self.ScalingGroupName_9 = None

	def getapiname(self):
		return 'ess.aliyuncs.com.DescribeScalingGroups.2014-08-28'

	def getTranslateParas(self):
		return {'ScalingGroupId_8':'ScalingGroupId.8','ScalingGroupId_9':'ScalingGroupId.9','ScalingGroupId_6':'ScalingGroupId.6','ScalingGroupId_7':'ScalingGroupId.7','ScalingGroupId_4':'ScalingGroupId.4','ScalingGroupId_5':'ScalingGroupId.5','ScalingGroupId_2':'ScalingGroupId.2','ScalingGroupId_3':'ScalingGroupId.3','ScalingGroupId_1':'ScalingGroupId.1','ScalingGroupId_20':'ScalingGroupId.20','ScalingGroupName_13':'ScalingGroupName.13','ScalingGroupName_14':'ScalingGroupName.14','ScalingGroupName_11':'ScalingGroupName.11','ScalingGroupName_12':'ScalingGroupName.12','ScalingGroupName_10':'ScalingGroupName.10','ScalingGroupName_19':'ScalingGroupName.19','ScalingGroupName_16':'ScalingGroupName.16','ScalingGroupName_15':'ScalingGroupName.15','ScalingGroupName_18':'ScalingGroupName.18','ScalingGroupName_17':'ScalingGroupName.17','ScalingGroupId_15':'ScalingGroupId.15','ScalingGroupId_16':'ScalingGroupId.16','ScalingGroupId_17':'ScalingGroupId.17','ScalingGroupId_18':'ScalingGroupId.18','ScalingGroupId_19':'ScalingGroupId.19','ScalingGroupName_7':'ScalingGroupName.7','ScalingGroupName_6':'ScalingGroupName.6','ScalingGroupName_5':'ScalingGroupName.5','ScalingGroupId_10':'ScalingGroupId.10','ScalingGroupName_4':'ScalingGroupName.4','ScalingGroupId_12':'ScalingGroupId.12','ScalingGroupId_11':'ScalingGroupId.11','ScalingGroupName_9':'ScalingGroupName.9','ScalingGroupId_14':'ScalingGroupId.14','ScalingGroupName_8':'ScalingGroupName.8','ScalingGroupId_13':'ScalingGroupId.13','ScalingGroupName_3':'ScalingGroupName.3','ScalingGroupName_2':'ScalingGroupName.2','ScalingGroupName_20':'ScalingGroupName.20','ScalingGroupName_1':'ScalingGroupName.1'}
