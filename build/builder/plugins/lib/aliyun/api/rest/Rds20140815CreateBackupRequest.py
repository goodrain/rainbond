'''
Created by auto_sdk on 2015.04.21
'''
from aliyun.api.base import RestApi
class Rds20140815CreateBackupRequest(RestApi):
	def __init__(self,domain='rds.aliyuncs.com',port=80):
		RestApi.__init__(self,domain, port)
		self.BackupMethod = None
		self.BackupType = None
		self.DBInstanceId = None

	def getapiname(self):
		return 'rds.aliyuncs.com.CreateBackup.2014-08-15'
