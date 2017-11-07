'''
Created by auto_sdk on 2015.04.21
'''
from aliyun.api.base import RestApi
class Ess20140828ModifyScalingRuleRequest(RestApi):
	def __init__(self,domain='ess.aliyuncs.com',port=80):
		RestApi.__init__(self,domain, port)
		self.AdjustmentType = None
		self.AdjustmentValue = None
		self.Cooldown = None
		self.ScalingRuleId = None
		self.ScalingRuleName = None

	def getapiname(self):
		return 'ess.aliyuncs.com.ModifyScalingRule.2014-08-28'
