'''
Created by auto_sdk on 2015.04.21
'''
from aliyun.api.base import RestApi
class Mts20140618QueryMediaSnapshotListRequest(RestApi):
	def __init__(self,domain='mts.aliyuncs.com',port=80):
		RestApi.__init__(self,domain, port)
		self.MediaId = None
		self.PageNumber = None
		self.PageSize = None

	def getapiname(self):
		return 'mts.aliyuncs.com.QueryMediaSnapshotList.2014-06-18'
