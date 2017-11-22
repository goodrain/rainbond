'''
Created by auto_sdk on 2015.04.21
'''
from aliyun.api.base import RestApi
class Mkvstore20150301DescribeHistoryMonitorValuesRequest(RestApi):
	def __init__(self,domain='m-kvstore.aliyuncs.com',port=80):
		RestApi.__init__(self,domain, port)
		self.EndTime = None
		self.InstanceId = None
		self.IntervalForHistory = None
		self.StartTime = None

	def getapiname(self):
		return 'm-kvstore.aliyuncs.com.DescribeHistoryMonitorValues.2015-03-01'
