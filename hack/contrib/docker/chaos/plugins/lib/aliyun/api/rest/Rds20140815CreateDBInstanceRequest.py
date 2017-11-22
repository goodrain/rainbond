'''
Created by auto_sdk on 2015.04.21
'''
from aliyun.api.base import RestApi
class Rds20140815CreateDBInstanceRequest(RestApi):
	def __init__(self,domain='rds.aliyuncs.com',port=80):
		RestApi.__init__(self,domain, port)
		self.ClientToken = None
		self.ConnectionMode = None
		self.DBInstanceClass = None
		self.DBInstanceDescription = None
		self.DBInstanceNetType = None
		self.DBInstanceStorage = None
		self.Engine = None
		self.EngineVersion = None
		self.InstanceNetworkType = None
		self.PayType = None
		self.PrivateIpAddress = None
		self.RegionId = None
		self.SecurityIPList = None
		self.VPCId = None
		self.VSwitchId = None
		self.ZoneId = None

	def getapiname(self):
		return 'rds.aliyuncs.com.CreateDBInstance.2014-08-15'
