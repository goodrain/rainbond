'''
Created by auto_sdk on 2015.04.21
'''
from aliyun.api.base import RestApi
class Rds20140815SwitchDBInstanceNetTypeRequest(RestApi):
	def __init__(self,domain='rds.aliyuncs.com',port=80):
		RestApi.__init__(self,domain, port)
		self.ConnectionStringPrefix = None
		self.DBInstanceId = None
		self.Port = None

	def getapiname(self):
		return 'rds.aliyuncs.com.SwitchDBInstanceNetType.2014-08-15'
