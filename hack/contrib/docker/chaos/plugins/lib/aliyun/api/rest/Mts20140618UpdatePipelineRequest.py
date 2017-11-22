'''
Created by auto_sdk on 2015.04.21
'''
from aliyun.api.base import RestApi
class Mts20140618UpdatePipelineRequest(RestApi):
	def __init__(self,domain='mts.aliyuncs.com',port=80):
		RestApi.__init__(self,domain, port)
		self.Name = None
		self.PipelineId = None
		self.State = None

	def getapiname(self):
		return 'mts.aliyuncs.com.UpdatePipeline.2014-06-18'
