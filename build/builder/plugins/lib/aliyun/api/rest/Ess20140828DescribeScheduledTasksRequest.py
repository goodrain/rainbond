'''
Created by auto_sdk on 2015.04.21
'''
from aliyun.api.base import RestApi
class Ess20140828DescribeScheduledTasksRequest(RestApi):
	def __init__(self,domain='ess.aliyuncs.com',port=80):
		RestApi.__init__(self,domain, port)
		self.PageNumber = None
		self.PageSize = None
		self.RegionId = None
		self.ScheduledAction_1 = None
		self.ScheduledAction_10 = None
		self.ScheduledAction_11 = None
		self.ScheduledAction_12 = None
		self.ScheduledAction_13 = None
		self.ScheduledAction_14 = None
		self.ScheduledAction_15 = None
		self.ScheduledAction_16 = None
		self.ScheduledAction_17 = None
		self.ScheduledAction_18 = None
		self.ScheduledAction_19 = None
		self.ScheduledAction_2 = None
		self.ScheduledAction_20 = None
		self.ScheduledAction_3 = None
		self.ScheduledAction_4 = None
		self.ScheduledAction_5 = None
		self.ScheduledAction_6 = None
		self.ScheduledAction_7 = None
		self.ScheduledAction_8 = None
		self.ScheduledAction_9 = None
		self.ScheduledTaskId_1 = None
		self.ScheduledTaskId_10 = None
		self.ScheduledTaskId_11 = None
		self.ScheduledTaskId_12 = None
		self.ScheduledTaskId_13 = None
		self.ScheduledTaskId_14 = None
		self.ScheduledTaskId_15 = None
		self.ScheduledTaskId_16 = None
		self.ScheduledTaskId_17 = None
		self.ScheduledTaskId_18 = None
		self.ScheduledTaskId_19 = None
		self.ScheduledTaskId_2 = None
		self.ScheduledTaskId_20 = None
		self.ScheduledTaskId_3 = None
		self.ScheduledTaskId_4 = None
		self.ScheduledTaskId_5 = None
		self.ScheduledTaskId_6 = None
		self.ScheduledTaskId_7 = None
		self.ScheduledTaskId_8 = None
		self.ScheduledTaskId_9 = None
		self.ScheduledTaskName_1 = None
		self.ScheduledTaskName_10 = None
		self.ScheduledTaskName_11 = None
		self.ScheduledTaskName_12 = None
		self.ScheduledTaskName_13 = None
		self.ScheduledTaskName_14 = None
		self.ScheduledTaskName_15 = None
		self.ScheduledTaskName_16 = None
		self.ScheduledTaskName_17 = None
		self.ScheduledTaskName_18 = None
		self.ScheduledTaskName_19 = None
		self.ScheduledTaskName_2 = None
		self.ScheduledTaskName_20 = None
		self.ScheduledTaskName_3 = None
		self.ScheduledTaskName_4 = None
		self.ScheduledTaskName_5 = None
		self.ScheduledTaskName_6 = None
		self.ScheduledTaskName_7 = None
		self.ScheduledTaskName_8 = None
		self.ScheduledTaskName_9 = None

	def getapiname(self):
		return 'ess.aliyuncs.com.DescribeScheduledTasks.2014-08-28'

	def getTranslateParas(self):
		return {'ScheduledTaskName_1':'ScheduledTaskName.1','ScheduledTaskName_5':'ScheduledTaskName.5','ScheduledTaskName_4':'ScheduledTaskName.4','ScheduledTaskName_3':'ScheduledTaskName.3','ScheduledTaskName_2':'ScheduledTaskName.2','ScheduledTaskId_1':'ScheduledTaskId.1','ScheduledTaskName_9':'ScheduledTaskName.9','ScheduledTaskName_8':'ScheduledTaskName.8','ScheduledTaskName_7':'ScheduledTaskName.7','ScheduledTaskName_6':'ScheduledTaskName.6','ScheduledTaskId_9':'ScheduledTaskId.9','ScheduledTaskId_8':'ScheduledTaskId.8','ScheduledTaskId_7':'ScheduledTaskId.7','ScheduledTaskId_6':'ScheduledTaskId.6','ScheduledTaskId_5':'ScheduledTaskId.5','ScheduledTaskId_4':'ScheduledTaskId.4','ScheduledTaskId_3':'ScheduledTaskId.3','ScheduledTaskId_2':'ScheduledTaskId.2','ScheduledAction_13':'ScheduledAction.13','ScheduledAction_14':'ScheduledAction.14','ScheduledAction_11':'ScheduledAction.11','ScheduledAction_12':'ScheduledAction.12','ScheduledAction_10':'ScheduledAction.10','ScheduledTaskId_20':'ScheduledTaskId.20','ScheduledTaskName_10':'ScheduledTaskName.10','ScheduledTaskName_12':'ScheduledTaskName.12','ScheduledTaskName_11':'ScheduledTaskName.11','ScheduledTaskName_14':'ScheduledTaskName.14','ScheduledTaskName_13':'ScheduledTaskName.13','ScheduledTaskName_16':'ScheduledTaskName.16','ScheduledTaskName_15':'ScheduledTaskName.15','ScheduledTaskName_18':'ScheduledTaskName.18','ScheduledTaskName_17':'ScheduledTaskName.17','ScheduledAction_20':'ScheduledAction.20','ScheduledTaskName_19':'ScheduledTaskName.19','ScheduledTaskId_12':'ScheduledTaskId.12','ScheduledAction_16':'ScheduledAction.16','ScheduledTaskId_11':'ScheduledTaskId.11','ScheduledAction_15':'ScheduledAction.15','ScheduledTaskId_10':'ScheduledTaskId.10','ScheduledAction_18':'ScheduledAction.18','ScheduledAction_17':'ScheduledAction.17','ScheduledTaskId_16':'ScheduledTaskId.16','ScheduledAction_19':'ScheduledAction.19','ScheduledTaskId_15':'ScheduledTaskId.15','ScheduledTaskId_14':'ScheduledTaskId.14','ScheduledTaskId_13':'ScheduledTaskId.13','ScheduledTaskId_19':'ScheduledTaskId.19','ScheduledTaskId_18':'ScheduledTaskId.18','ScheduledTaskId_17':'ScheduledTaskId.17','ScheduledAction_8':'ScheduledAction.8','ScheduledTaskName_20':'ScheduledTaskName.20','ScheduledAction_9':'ScheduledAction.9','ScheduledAction_6':'ScheduledAction.6','ScheduledAction_7':'ScheduledAction.7','ScheduledAction_4':'ScheduledAction.4','ScheduledAction_5':'ScheduledAction.5','ScheduledAction_2':'ScheduledAction.2','ScheduledAction_3':'ScheduledAction.3','ScheduledAction_1':'ScheduledAction.1'}
