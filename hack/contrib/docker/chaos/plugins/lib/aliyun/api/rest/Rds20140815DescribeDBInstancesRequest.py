'''
Created by auto_sdk on 2015.04.21
'''
from aliyun.api.base import RestApi
class Rds20140815DescribeDBInstancesRequest(RestApi):
	def __init__(self,domain='rds.aliyuncs.com',port=80):
		RestApi.__init__(self,domain, port)
		self.ConnectionMode = None
		self.DBInstanceId = None
		self.DBInstanceStatus = None
		self.DBInstanceType = None
		self.Engine = None
		self.InstanceNetworkType = None
		self.PageNumber = None
		self.PageSize = None
		self.RegionId = None
		self.SearchKey = None

	def getapiname(self):
		return 'rds.aliyuncs.com.DescribeDBInstances.2014-08-15'
