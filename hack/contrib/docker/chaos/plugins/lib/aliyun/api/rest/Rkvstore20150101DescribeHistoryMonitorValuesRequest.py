'''
Created by auto_sdk on 2015.04.21
'''
from aliyun.api.base import RestApi
class Rkvstore20150101DescribeHistoryMonitorValuesRequest(RestApi):
	def __init__(self,domain='r-kvstore.aliyuncs.com',port=80):
		RestApi.__init__(self,domain, port)
		self.EndTime = None
		self.InstanceId = None
		self.IntervalForHistory = None
		self.MonitorKeys = None
		self.StartTime = None

	def getapiname(self):
		return 'r-kvstore.aliyuncs.com.DescribeHistoryMonitorValues.2015-01-01'
