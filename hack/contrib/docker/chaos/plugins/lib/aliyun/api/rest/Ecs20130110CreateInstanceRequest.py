'''
Created by auto_sdk on 2015.04.21
'''
from aliyun.api.base import RestApi
class Ecs20130110CreateInstanceRequest(RestApi):
	def __init__(self,domain='ecs.aliyuncs.com',port=80):
		RestApi.__init__(self,domain, port)
		self.ClientToken = None
		self.DataDisk_1_Category = None
		self.DataDisk_1_Size = None
		self.DataDisk_1_SnapshotId = None
		self.DataDisk_2_Category = None
		self.DataDisk_2_Size = None
		self.DataDisk_2_SnapshotId = None
		self.DataDisk_3_Category = None
		self.DataDisk_3_Size = None
		self.DataDisk_3_SnapshotId = None
		self.DataDisk_4_Category = None
		self.DataDisk_4_Size = None
		self.DataDisk_4_SnapshotId = None
		self.HostName = None
		self.ImageId = None
		self.InnerIpAddress = None
		self.InstanceName = None
		self.InstanceType = None
		self.InternetChargeType = None
		self.InternetMaxBandwidthIn = None
		self.InternetMaxBandwidthOut = None
		self.NodeControllerId = None
		self.Password = None
		self.RegionId = None
		self.SecurityGroupId = None
		self.SystemDisk_Category = None
		self.VlanId = None
		self.ZoneId = None

	def getapiname(self):
		return 'ecs.aliyuncs.com.CreateInstance.2013-01-10'

	def getTranslateParas(self):
		return {'DataDisk_3_Category':'DataDisk.3.Category','DataDisk_2_SnapshotId':'DataDisk.2.SnapshotId','DataDisk_4_Size':'DataDisk.4.Size','DataDisk_1_Size':'DataDisk.1.Size','DataDisk_3_SnapshotId':'DataDisk.3.SnapshotId','DataDisk_1_SnapshotId':'DataDisk.1.SnapshotId','SystemDisk_Category':'SystemDisk.Category','DataDisk_2_Size':'DataDisk.2.Size','DataDisk_4_Category':'DataDisk.4.Category','DataDisk_3_Size':'DataDisk.3.Size','DataDisk_1_Category':'DataDisk.1.Category','DataDisk_4_SnapshotId':'DataDisk.4.SnapshotId','DataDisk_2_Category':'DataDisk.2.Category'}
