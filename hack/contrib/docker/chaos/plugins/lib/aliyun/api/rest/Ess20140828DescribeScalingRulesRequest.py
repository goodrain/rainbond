'''
Created by auto_sdk on 2015.04.21
'''
from aliyun.api.base import RestApi
class Ess20140828DescribeScalingRulesRequest(RestApi):
	def __init__(self,domain='ess.aliyuncs.com',port=80):
		RestApi.__init__(self,domain, port)
		self.PageNumber = None
		self.PageSize = None
		self.RegionId = None
		self.ScalingGroupId = None
		self.ScalingRuleAri_1 = None
		self.ScalingRuleAri_10 = None
		self.ScalingRuleAri_2 = None
		self.ScalingRuleAri_3 = None
		self.ScalingRuleAri_4 = None
		self.ScalingRuleAri_5 = None
		self.ScalingRuleAri_6 = None
		self.ScalingRuleAri_7 = None
		self.ScalingRuleAri_8 = None
		self.ScalingRuleAri_9 = None
		self.ScalingRuleId_1 = None
		self.ScalingRuleId_10 = None
		self.ScalingRuleId_2 = None
		self.ScalingRuleId_3 = None
		self.ScalingRuleId_4 = None
		self.ScalingRuleId_5 = None
		self.ScalingRuleId_6 = None
		self.ScalingRuleId_7 = None
		self.ScalingRuleId_8 = None
		self.ScalingRuleId_9 = None
		self.ScalingRuleName_1 = None
		self.ScalingRuleName_10 = None
		self.ScalingRuleName_2 = None
		self.ScalingRuleName_3 = None
		self.ScalingRuleName_4 = None
		self.ScalingRuleName_5 = None
		self.ScalingRuleName_6 = None
		self.ScalingRuleName_7 = None
		self.ScalingRuleName_8 = None
		self.ScalingRuleName_9 = None

	def getapiname(self):
		return 'ess.aliyuncs.com.DescribeScalingRules.2014-08-28'

	def getTranslateParas(self):
		return {'ScalingRuleName_3':'ScalingRuleName.3','ScalingRuleName_4':'ScalingRuleName.4','ScalingRuleName_1':'ScalingRuleName.1','ScalingRuleName_2':'ScalingRuleName.2','ScalingRuleName_7':'ScalingRuleName.7','ScalingRuleName_8':'ScalingRuleName.8','ScalingRuleName_5':'ScalingRuleName.5','ScalingRuleName_6':'ScalingRuleName.6','ScalingRuleId_10':'ScalingRuleId.10','ScalingRuleName_9':'ScalingRuleName.9','ScalingRuleAri_8':'ScalingRuleAri.8','ScalingRuleAri_9':'ScalingRuleAri.9','ScalingRuleAri_6':'ScalingRuleAri.6','ScalingRuleAri_7':'ScalingRuleAri.7','ScalingRuleAri_4':'ScalingRuleAri.4','ScalingRuleAri_5':'ScalingRuleAri.5','ScalingRuleAri_2':'ScalingRuleAri.2','ScalingRuleAri_3':'ScalingRuleAri.3','ScalingRuleAri_1':'ScalingRuleAri.1','ScalingRuleName_10':'ScalingRuleName.10','ScalingRuleAri_10':'ScalingRuleAri.10','ScalingRuleId_9':'ScalingRuleId.9','ScalingRuleId_8':'ScalingRuleId.8','ScalingRuleId_7':'ScalingRuleId.7','ScalingRuleId_6':'ScalingRuleId.6','ScalingRuleId_5':'ScalingRuleId.5','ScalingRuleId_4':'ScalingRuleId.4','ScalingRuleId_3':'ScalingRuleId.3','ScalingRuleId_2':'ScalingRuleId.2','ScalingRuleId_1':'ScalingRuleId.1'}
