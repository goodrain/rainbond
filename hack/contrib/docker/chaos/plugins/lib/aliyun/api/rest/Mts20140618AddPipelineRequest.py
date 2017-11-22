'''
Created by auto_sdk on 2015.04.21
'''
from aliyun.api.base import RestApi
class Mts20140618AddPipelineRequest(RestApi):
	def __init__(self,domain='mts.aliyuncs.com',port=80):
		RestApi.__init__(self,domain, port)
		self.Name = None

	def getapiname(self):
		return 'mts.aliyuncs.com.AddPipeline.2014-06-18'
