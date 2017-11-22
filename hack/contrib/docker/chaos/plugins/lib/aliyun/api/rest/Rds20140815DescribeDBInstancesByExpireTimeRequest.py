'''
Created by auto_sdk on 2015.04.21
'''
from aliyun.api.base import RestApi
class Rds20140815DescribeDBInstancesByExpireTimeRequest(RestApi):
	def __init__(self,domain='rds.aliyuncs.com',port=80):
		RestApi.__init__(self,domain, port)
		self.ExpirePeriod = None
		self.PageNumber = None
		self.PageSize = None

	def getapiname(self):
		return 'rds.aliyuncs.com.DescribeDBInstancesByExpireTime.2014-08-15'
