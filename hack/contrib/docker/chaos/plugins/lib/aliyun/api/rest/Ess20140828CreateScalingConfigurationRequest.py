'''
Created by auto_sdk on 2015.04.21
'''
from aliyun.api.base import RestApi
class Ess20140828CreateScalingConfigurationRequest(RestApi):
	def __init__(self,domain='ess.aliyuncs.com',port=80):
		RestApi.__init__(self,domain, port)
		self.DataDisk_1_Category = None
		self.DataDisk_1_Device = None
		self.DataDisk_1_Size = None
		self.DataDisk_1_SnapshotId = None
		self.DataDisk_2_Category = None
		self.DataDisk_2_Device = None
		self.DataDisk_2_Size = None
		self.DataDisk_2_SnapshotId = None
		self.DataDisk_3_Category = None
		self.DataDisk_3_Device = None
		self.DataDisk_3_Size = None
		self.DataDisk_3_SnapshotId = None
		self.DataDisk_4_Category = None
		self.DataDisk_4_Device = None
		self.DataDisk_4_Size = None
		self.DataDisk_4_SnapshotId = None
		self.ImageId = None
		self.InstanceType = None
		self.InternetChargeType = None
		self.InternetMaxBandwidthIn = None
		self.InternetMaxBandwidthOut = None
		self.ScalingConfigurationName = None
		self.ScalingGroupId = None
		self.SecurityGroupId = None
		self.SystemDisk_Category = None

	def getapiname(self):
		return 'ess.aliyuncs.com.CreateScalingConfiguration.2014-08-28'

	def getTranslateParas(self):
		return {'DataDisk_4_Device':'DataDisk.4.Device','DataDisk_3_Category':'DataDisk.3.Category','DataDisk_3_Device':'DataDisk.3.Device','DataDisk_2_SnapshotId':'DataDisk.2.SnapshotId','DataDisk_4_Size':'DataDisk.4.Size','DataDisk_1_Device':'DataDisk.1.Device','DataDisk_1_Size':'DataDisk.1.Size','DataDisk_3_SnapshotId':'DataDisk.3.SnapshotId','DataDisk_1_SnapshotId':'DataDisk.1.SnapshotId','SystemDisk_Category':'SystemDisk.Category','DataDisk_2_Size':'DataDisk.2.Size','DataDisk_4_Category':'DataDisk.4.Category','DataDisk_3_Size':'DataDisk.3.Size','DataDisk_1_Category':'DataDisk.1.Category','DataDisk_4_SnapshotId':'DataDisk.4.SnapshotId','DataDisk_2_Device':'DataDisk.2.Device','DataDisk_2_Category':'DataDisk.2.Category'}
