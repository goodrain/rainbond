'''
Created by auto_sdk on 2015.04.21
'''
from aliyun.api.base import RestApi
class Mts20140618DeleteTemplateRequest(RestApi):
	def __init__(self,domain='mts.aliyuncs.com',port=80):
		RestApi.__init__(self,domain, port)
		self.TemplateId = None

	def getapiname(self):
		return 'mts.aliyuncs.com.DeleteTemplate.2014-06-18'
