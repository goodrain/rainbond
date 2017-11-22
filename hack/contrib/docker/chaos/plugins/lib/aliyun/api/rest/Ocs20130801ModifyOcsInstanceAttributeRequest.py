'''
Created by auto_sdk on 2015.04.21
'''
from aliyun.api.base import RestApi
class Ocs20130801ModifyOcsInstanceAttributeRequest(RestApi):
	def __init__(self,domain='ocs.aliyuncs.com',port=80):
		RestApi.__init__(self,domain, port)
		self.NewPassword = None
		self.OcsInstanceId = None
		self.OcsInstanceName = None
		self.OldPassword = None

	def getapiname(self):
		return 'ocs.aliyuncs.com.ModifyOcsInstanceAttribute.2013-08-01'
