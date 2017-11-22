'''
Created by auto_sdk on 2015.04.21
'''
from aliyun.api.base import RestApi
class Mkvstore20150301DescribeMonitorValuesRequest(RestApi):
	def __init__(self,domain='m-kvstore.aliyuncs.com',port=80):
		RestApi.__init__(self,domain, port)
		self.InstanceIds = None
		self.MonitorKeys = None

	def getapiname(self):
		return 'm-kvstore.aliyuncs.com.DescribeMonitorValues.2015-03-01'
