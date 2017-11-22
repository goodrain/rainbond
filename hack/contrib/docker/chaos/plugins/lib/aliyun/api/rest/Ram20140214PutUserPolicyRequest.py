'''
Created by auto_sdk on 2015.04.21
'''
from aliyun.api.base import RestApi
class Ram20140214PutUserPolicyRequest(RestApi):
	def __init__(self,domain='ram.aliyuncs.com',port=80):
		RestApi.__init__(self,domain, port)
		self.AccountSpace = None
		self.PolicyDocument = None
		self.PolicyName = None
		self.UserName = None

	def getapiname(self):
		return 'ram.aliyuncs.com.PutUserPolicy.2014-02-14'
