'''
Created by auto_sdk on 2015.04.21
'''
from aliyun.api.base import RestApi
class Ocm20140820SingleSendMailRequest(RestApi):
	def __init__(self,domain='ocm.aliyuncs.com',port=80):
		RestApi.__init__(self,domain, port)
		self.addressType = None
		self.fromAddress = None
		self.htmlBody = None
		self.replyToAddress = None
		self.subject = None
		self.textBody = None
		self.toAddress = None

	def getapiname(self):
		return 'ocm.aliyuncs.com.SingleSendMail.2014-08-20'
