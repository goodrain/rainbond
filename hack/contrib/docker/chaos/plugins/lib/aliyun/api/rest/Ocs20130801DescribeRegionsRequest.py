'''
Created by auto_sdk on 2015.04.21
'''
from aliyun.api.base import RestApi
class Ocs20130801DescribeRegionsRequest(RestApi):
	def __init__(self,domain='ocs.aliyuncs.com',port=80):
		RestApi.__init__(self,domain, port)

	def getapiname(self):
		return 'ocs.aliyuncs.com.DescribeRegions.2013-08-01'
