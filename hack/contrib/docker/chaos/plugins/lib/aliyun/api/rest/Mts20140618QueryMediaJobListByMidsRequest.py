'''
Created by auto_sdk on 2015.04.21
'''
from aliyun.api.base import RestApi
class Mts20140618QueryMediaJobListByMidsRequest(RestApi):
	def __init__(self,domain='mts.aliyuncs.com',port=80):
		RestApi.__init__(self,domain, port)
		self.MediaIds = None
		self.State = None

	def getapiname(self):
		return 'mts.aliyuncs.com.QueryMediaJobListByMids.2014-06-18'
