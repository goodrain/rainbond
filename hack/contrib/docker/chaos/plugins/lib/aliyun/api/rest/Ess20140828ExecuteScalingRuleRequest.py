'''
Created by auto_sdk on 2015.04.21
'''
from aliyun.api.base import RestApi
class Ess20140828ExecuteScalingRuleRequest(RestApi):
	def __init__(self,domain='ess.aliyuncs.com',port=80):
		RestApi.__init__(self,domain, port)
		self.ClientToken = None
		self.ScalingRuleAri = None

	def getapiname(self):
		return 'ess.aliyuncs.com.ExecuteScalingRule.2014-08-28'
