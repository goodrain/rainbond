'''
Created by auto_sdk on 2015.04.21
'''
from aliyun.api.base import RestApi
class Ecs20140526CreateInstanceRequest(RestApi):
	def __init__(self,domain='ecs.aliyuncs.com',port=80):
		RestApi.__init__(self,domain, port)
		self.ClientToken = None
		self.ClusterId = None
		self.DataDisk_1_Category = None
		self.DataDisk_1_DeleteWithInstance = None
		self.DataDisk_1_Description = None
		self.DataDisk_1_Device = None
		self.DataDisk_1_DiskName = None
		self.DataDisk_1_Size = None
		self.DataDisk_1_SnapshotId = None
		self.DataDisk_2_Category = None
		self.DataDisk_2_DeleteWithInstance = None
		self.DataDisk_2_Description = None
		self.DataDisk_2_Device = None
		self.DataDisk_2_DiskName = None
		self.DataDisk_2_Size = None
		self.DataDisk_2_SnapshotId = None
		self.DataDisk_3_Category = None
		self.DataDisk_3_DeleteWithInstance = None
		self.DataDisk_3_Description = None
		self.DataDisk_3_Device = None
		self.DataDisk_3_DiskName = None
		self.DataDisk_3_Size = None
		self.DataDisk_3_SnapshotId = None
		self.DataDisk_4_Category = None
		self.DataDisk_4_DeleteWithInstance = None
		self.DataDisk_4_Description = None
		self.DataDisk_4_Device = None
		self.DataDisk_4_DiskName = None
		self.DataDisk_4_Size = None
		self.DataDisk_4_SnapshotId = None
		self.Description = None
		self.HostName = None
		self.ImageId = None
		self.InnerIpAddress = None
		self.InstanceName = None
		self.InstanceType = None
		self.InternetChargeType = None
		self.InternetMaxBandwidthIn = None
		self.InternetMaxBandwidthOut = None
		self.IoOptimized = None
		self.NodeControllerId = None
		self.Password = None
		self.PrivateIpAddress = None
		self.RegionId = None
		self.SecurityGroupId = None
		self.SystemDisk_Category = None
		self.SystemDisk_Description = None
		self.SystemDisk_DiskName = None
		self.VSwitchId = None
		self.VlanId = None
		self.ZoneId = None

	def getapiname(self):
		return 'ecs.aliyuncs.com.CreateInstance.2014-05-26'

	def getTranslateParas(self):
		return {'DataDisk_3_Description':'DataDisk.3.Description','DataDisk_1_Description':'DataDisk.1.Description','DataDisk_4_Size':'DataDisk.4.Size','DataDisk_1_Device':'DataDisk.1.Device','DataDisk_1_Size':'DataDisk.1.Size','SystemDisk_Category':'SystemDisk.Category','SystemDisk_DiskName':'SystemDisk.DiskName','DataDisk_2_Description':'DataDisk.2.Description','DataDisk_3_Size':'DataDisk.3.Size','DataDisk_1_Category':'DataDisk.1.Category','SystemDisk_Description':'SystemDisk.Description','DataDisk_4_SnapshotId':'DataDisk.4.SnapshotId','DataDisk_3_Category':'DataDisk.3.Category','DataDisk_3_Device':'DataDisk.3.Device','DataDisk_4_Device':'DataDisk.4.Device','DataDisk_2_SnapshotId':'DataDisk.2.SnapshotId','DataDisk_1_DiskName':'DataDisk.1.DiskName','DataDisk_4_DiskName':'DataDisk.4.DiskName','DataDisk_2_DeleteWithInstance':'DataDisk.2.DeleteWithInstance','DataDisk_3_SnapshotId':'DataDisk.3.SnapshotId','DataDisk_1_SnapshotId':'DataDisk.1.SnapshotId','DataDisk_2_Size':'DataDisk.2.Size','DataDisk_4_Category':'DataDisk.4.Category','DataDisk_1_DeleteWithInstance':'DataDisk.1.DeleteWithInstance','DataDisk_4_Description':'DataDisk.4.Description','DataDisk_3_DeleteWithInstance':'DataDisk.3.DeleteWithInstance','DataDisk_4_DeleteWithInstance':'DataDisk.4.DeleteWithInstance','DataDisk_2_Device':'DataDisk.2.Device','DataDisk_2_DiskName':'DataDisk.2.DiskName','DataDisk_3_DiskName':'DataDisk.3.DiskName','DataDisk_2_Category':'DataDisk.2.Category'}
