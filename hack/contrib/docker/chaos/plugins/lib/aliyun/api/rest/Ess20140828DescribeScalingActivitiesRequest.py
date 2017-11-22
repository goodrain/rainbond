'''
Created by auto_sdk on 2015.04.21
'''
from aliyun.api.base import RestApi
class Ess20140828DescribeScalingActivitiesRequest(RestApi):
	def __init__(self,domain='ess.aliyuncs.com',port=80):
		RestApi.__init__(self,domain, port)
		self.PageNumber = None
		self.PageSize = None
		self.RegionId = None
		self.ScalingActivityId_1 = None
		self.ScalingActivityId_10 = None
		self.ScalingActivityId_11 = None
		self.ScalingActivityId_12 = None
		self.ScalingActivityId_13 = None
		self.ScalingActivityId_14 = None
		self.ScalingActivityId_15 = None
		self.ScalingActivityId_16 = None
		self.ScalingActivityId_17 = None
		self.ScalingActivityId_18 = None
		self.ScalingActivityId_19 = None
		self.ScalingActivityId_2 = None
		self.ScalingActivityId_20 = None
		self.ScalingActivityId_3 = None
		self.ScalingActivityId_4 = None
		self.ScalingActivityId_5 = None
		self.ScalingActivityId_6 = None
		self.ScalingActivityId_7 = None
		self.ScalingActivityId_8 = None
		self.ScalingActivityId_9 = None
		self.ScalingGroupId = None
		self.StatusCode = None

	def getapiname(self):
		return 'ess.aliyuncs.com.DescribeScalingActivities.2014-08-28'

	def getTranslateParas(self):
		return {'ScalingActivityId_5':'ScalingActivityId.5','ScalingActivityId_13':'ScalingActivityId.13','ScalingActivityId_4':'ScalingActivityId.4','ScalingActivityId_14':'ScalingActivityId.14','ScalingActivityId_3':'ScalingActivityId.3','ScalingActivityId_15':'ScalingActivityId.15','ScalingActivityId_16':'ScalingActivityId.16','ScalingActivityId_2':'ScalingActivityId.2','ScalingActivityId_1':'ScalingActivityId.1','ScalingActivityId_17':'ScalingActivityId.17','ScalingActivityId_18':'ScalingActivityId.18','ScalingActivityId_19':'ScalingActivityId.19','ScalingActivityId_9':'ScalingActivityId.9','ScalingActivityId_8':'ScalingActivityId.8','ScalingActivityId_7':'ScalingActivityId.7','ScalingActivityId_6':'ScalingActivityId.6','ScalingActivityId_10':'ScalingActivityId.10','ScalingActivityId_12':'ScalingActivityId.12','ScalingActivityId_11':'ScalingActivityId.11','ScalingActivityId_20':'ScalingActivityId.20'}
