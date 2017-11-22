'''
Created by auto_sdk on 2015.04.21
'''
from aliyun.api.base import RestApi
class Ecs20140526AuthorizeSecurityGroupRequest(RestApi):
	def __init__(self,domain='ecs.aliyuncs.com',port=80):
		RestApi.__init__(self,domain, port)
		self.IpProtocol = None
		self.NicType = None
		self.Policy = None
		self.PortRange = None
		self.RegionId = None
		self.SecurityGroupId = None
		self.SourceCidrIp = None
		self.SourceGroupId = None
		self.SourceGroupOwnerAccount = None

	def getapiname(self):
		return 'ecs.aliyuncs.com.AuthorizeSecurityGroup.2014-05-26'
